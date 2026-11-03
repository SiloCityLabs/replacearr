// Package downloadmonitor polls download clients and imports completed files into library folders.
package downloadmonitor

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/downloadclient/qbittorrent"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// Notifier is optional callback into the API notification bus.
type Notifier interface {
	EmitDownloadComplete(ctx context.Context, payload map[string]any)
	EmitDownloadMoved(ctx context.Context, payload map[string]any)
}

// PluginDownloads lists downloads from enabled download-client plugins.
type PluginDownloads interface {
	ListPluginDownloads(ctx context.Context) ([]pluginsdk.DownloadItem, error)
}

// Monitor polls qBittorrent (and plugins) and runs completed-download import.
type Monitor struct {
	store    *db.Store
	plugins  PluginDownloads
	notify   Notifier
	interval time.Duration
}

// New creates a Monitor with a default 15s tick.
func New(store *db.Store, plugins PluginDownloads, notify Notifier) *Monitor {
	return &Monitor{
		store:    store,
		plugins:  plugins,
		notify:   notify,
		interval: 15 * time.Second,
	}
}

// Run blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	m.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	appCfg, err := m.store.GetAppConfig()
	if err != nil {
		return
	}
	cdh := appCfg.DownloadClientOptions.EnableCompletedDownloadHandling

	queue, err := m.store.ListQueueItems("")
	if err != nil {
		return
	}
	active := make([]domain.QueueItem, 0, len(queue))
	for _, q := range queue {
		if q.TrackedDownloadState == "imported" || q.Status == "completed" && q.TrackedDownloadState == "imported" {
			continue
		}
		if q.DownloadID == "" {
			continue
		}
		active = append(active, q)
	}
	if len(active) == 0 {
		return
	}

	byID := map[string]trackedDownload{}
	clients, _ := m.store.ListDownloadClients()
	for _, dc := range clients {
		if !dc.Enable || !strings.EqualFold(dc.Implementation, "qBittorrent") {
			continue
		}
		cli := qbittorrent.NewFromDownloadClient(dc)
		items, err := cli.List(ctx)
		if err != nil {
			log.Printf("downloadmonitor: qBittorrent list failed (%s): %v", dc.Name, err)
			continue
		}
		for _, it := range items {
			byID[it.Hash] = trackedDownload{
				ID:          it.Hash,
				Title:       it.Name,
				Completed:   qbittorrent.IsCompleted(it.State, it.Progress),
				Progress:    it.Progress,
				Size:        it.Size,
				SizeLeft:    it.AmountLeft,
				OutputPath:  firstNonEmpty(it.ContentPath, it.SavePath),
				ContentPath: it.ContentPath,
				Client:      cli,
				RemoveDone:  dc.RemoveCompletedDownloads,
				ClientName:  dc.Name,
			}
		}
	}
	if m.plugins != nil {
		if items, err := m.plugins.ListPluginDownloads(ctx); err == nil {
			for _, it := range items {
				byID[strings.ToLower(it.ID)] = trackedDownload{
					ID:          strings.ToLower(it.ID),
					Title:       it.Title,
					Completed:   strings.EqualFold(it.Status, "completed") || it.Progress >= 0.999,
					Progress:    it.Progress,
					Size:        it.Size,
					SizeLeft:    it.SizeLeft,
					OutputPath:  firstNonEmpty(it.ContentPath, it.OutputPath),
					ContentPath: it.ContentPath,
				}
			}
		}
	}

	for _, q := range active {
		td, ok := byID[strings.ToLower(q.DownloadID)]
		if !ok {
			// pending hash or client not ready
			continue
		}
		q.Size = td.Size
		q.Sizeleft = td.SizeLeft
		mapped := applyRemotePathMapping(appCfg.RemotePathMappings, td.OutputPath)
		if mapped != "" {
			q.OutputPath = mapped
		}
		if !td.Completed {
			q.Status = "downloading"
			q.TrackedDownloadState = "downloading"
			_ = m.store.UpdateQueueItem(q)
			continue
		}
		q.Status = "completed"
		q.TrackedDownloadState = "importPending"
		q.Sizeleft = 0
		_ = m.store.UpdateQueueItem(q)

		if !cdh {
			continue
		}
		if q.SeriesID == 0 && q.MovieID == 0 {
			q.TrackedDownloadStatus = "warning"
			q.TrackedDownloadState = "importBlocked"
			_ = m.store.UpdateQueueItem(q)
			continue
		}
		if err := m.importCompleted(ctx, appCfg, q); err != nil {
			log.Printf("downloadmonitor: import failed for queue %d: %v", q.ID, err)
			q.TrackedDownloadStatus = "error"
			q.TrackedDownloadState = "importBlocked"
			_ = m.store.UpdateQueueItem(q)
			continue
		}
		q.TrackedDownloadStatus = "ok"
		q.TrackedDownloadState = "imported"
		q.Status = "completed"
		_ = m.store.UpdateQueueItem(q)
		if td.RemoveDone && td.Client != nil {
			_ = td.Client.Remove(ctx, td.ID, false)
		}
	}
}

type trackedDownload struct {
	ID          string
	Title       string
	Completed   bool
	Progress    float64
	Size        int64
	SizeLeft    int64
	OutputPath  string
	ContentPath string
	Client      *qbittorrent.Client
	RemoveDone  bool
	ClientName  string
}

func (m *Monitor) importCompleted(ctx context.Context, appCfg domain.AppConfig, q domain.QueueItem) error {
	srcRoot := q.OutputPath
	if srcRoot == "" {
		return fmt.Errorf("no output path")
	}
	video, err := findPrimaryVideo(srcRoot)
	if err != nil {
		return err
	}
	dest, err := m.destinationPath(appCfg, q, filepath.Ext(video))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := importFile(video, dest, appCfg.MediaManagement.CopyUsingHardlinks); err != nil {
		return err
	}

	if q.MovieID > 0 {
		if mv, err := m.store.GetMovieByID(q.MovieID); err == nil && mv != nil {
			mv.HasFile = true
			mv.Path = filepath.Dir(dest)
			_, _ = m.store.UpdateMovie(*mv)
		} else {
			_ = m.store.MarkMovieHasFile(q.MovieID, true)
		}
	}
	if q.SeriesID > 0 {
		if q.EpisodeID > 0 {
			_ = m.store.MarkEpisodeHasFile(q.EpisodeID, true)
		} else if season, ep, ok := parseSeasonEpisode(q.Title); ok {
			_ = m.store.MarkEpisodeHasFileByNumbers(q.SeriesID, season, ep, true)
		}
	}

	_ = m.store.CreateHistory(domain.HistoryRecord{
		MediaType:   q.MediaType,
		SeriesID:    q.SeriesID,
		EpisodeID:   q.EpisodeID,
		MovieID:     q.MovieID,
		SourceTitle: q.Title,
		Quality:     map[string]any{"quality": map[string]any{"name": "Unknown"}},
		EventType:   "downloadFolderImported",
		Data: map[string]any{
			"sourcePath": video,
			"destPath":   dest,
			"downloadId": q.DownloadID,
		},
	})

	if m.notify != nil {
		m.notify.EmitDownloadComplete(ctx, map[string]any{
			"title": q.Title, "queueId": q.ID, "path": dest, "mediaType": q.MediaType,
		})
		m.notify.EmitDownloadMoved(ctx, map[string]any{
			"title": q.Title, "queueId": q.ID, "sourcePath": video, "destPath": dest,
		})
	}
	return nil
}

func (m *Monitor) destinationPath(appCfg domain.AppConfig, q domain.QueueItem, ext string) (string, error) {
	if ext == "" {
		ext = ".mkv"
	}
	if q.MovieID > 0 {
		mv, err := m.store.GetMovieByID(q.MovieID)
		if err != nil || mv == nil {
			return "", fmt.Errorf("movie %d not found", q.MovieID)
		}
		base := strings.TrimSpace(mv.Path)
		if base == "" {
			root := firstRoot(appCfg, m.store, "movie")
			folder := fmt.Sprintf("%s (%d)", sanitizeName(mv.Title), mv.Year)
			if mv.Year == 0 {
				folder = sanitizeName(mv.Title)
			}
			base = filepath.Join(root, folder)
		}
		file := sanitizeName(mv.Title)
		if mv.Year > 0 {
			file = fmt.Sprintf("%s (%d)", sanitizeName(mv.Title), mv.Year)
		}
		return filepath.Join(base, file+ext), nil
	}
	if q.SeriesID > 0 {
		sr, err := m.store.GetSeriesByID(q.SeriesID)
		if err != nil || sr == nil {
			return "", fmt.Errorf("series %d not found", q.SeriesID)
		}
		base := strings.TrimSpace(sr.Path)
		if base == "" {
			root := firstRoot(appCfg, m.store, "tv")
			base = filepath.Join(root, sanitizeName(sr.Title))
		}
		season, ep := 1, 1
		if q.EpisodeID > 0 {
			// keep defaults; numbers may come from title parse below
		}
		if s, e, ok := parseSeasonEpisode(q.Title); ok {
			season, ep = s, e
		}
		seasonDir := fmt.Sprintf("Season %02d", season)
		file := fmt.Sprintf("%s - S%02dE%02d%s", sanitizeName(sr.Title), season, ep, ext)
		return filepath.Join(base, seasonDir, file), nil
	}
	return "", fmt.Errorf("no library association")
}

func firstRoot(appCfg domain.AppConfig, store *db.Store, mediaType string) string {
	roots, err := store.ListRootFolders(mediaType)
	if err == nil && len(roots) > 0 {
		return roots[0].Path
	}
	roots, err = store.ListRootFolders("")
	if err == nil && len(roots) > 0 {
		return roots[0].Path
	}
	_ = appCfg
	return filepath.Join(".", "media", mediaType)
}

func applyRemotePathMapping(mappings []domain.RemotePathMapping, path string) string {
	if path == "" || len(mappings) == 0 {
		return path
	}
	for _, m := range mappings {
		remote := strings.TrimRight(m.RemotePath, `/\`)
		local := strings.TrimRight(m.LocalPath, `/\`)
		if remote != "" && (strings.HasPrefix(path, remote) || strings.HasPrefix(path, remote+"/")) {
			return local + path[len(remote):]
		}
	}
	return path
}

func findPrimaryVideo(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		if isVideo(root) {
			return root, nil
		}
		return "", fmt.Errorf("not a video file: %s", root)
	}
	var best string
	var bestSize int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !isVideo(path) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if fi.Size() > bestSize {
			bestSize = fi.Size()
			best = path
		}
		return nil
	})
	if best == "" {
		return "", fmt.Errorf("no video files under %s", root)
	}
	return best, nil
}

func isVideo(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".avi", ".m4v", ".ts", ".m2ts", ".wmv", ".mov":
		return true
	default:
		return false
	}
}

func importFile(src, dest string, hardlink bool) error {
	if src == dest {
		return nil
	}
	if hardlink {
		if err := os.Link(src, dest); err == nil {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer(`/`, "-", `\`, "-", `:`, " -", `*`, "", `?`, "", `"`, "", `<`, "", `>`, "", `|`, "")
	return strings.TrimSpace(replacer.Replace(name))
}

func parseSeasonEpisode(title string) (int, int, bool) {
	upper := strings.ToUpper(title)
	// S01E02
	for i := 0; i < len(upper)-5; i++ {
		if upper[i] == 'S' && upper[i+3] == 'E' {
			s := atoi2(upper[i+1 : i+3])
			e := atoi2(upper[i+4 : i+6])
			if s >= 0 && e > 0 {
				return s, e, true
			}
		}
	}
	return 0, 0, false
}

func atoi2(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
