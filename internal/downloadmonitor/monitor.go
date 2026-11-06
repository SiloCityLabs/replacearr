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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/downloadclient/deluge"
	"github.com/replacearr/replacearr/internal/downloadclient/nzbget"
	"github.com/replacearr/replacearr/internal/downloadclient/qbittorrent"
	"github.com/replacearr/replacearr/internal/downloadclient/sabnzbd"
	"github.com/replacearr/replacearr/internal/downloadclient/transmission"
	"github.com/replacearr/replacearr/internal/mediafile"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

// downloadRemover deletes a finished torrent from its client (optional).
type downloadRemover interface {
	Remove(ctx context.Context, id string, deleteFiles bool) error
}

// Notifier is optional callback into the API notification bus.
type Notifier interface {
	EmitDownloadComplete(ctx context.Context, payload map[string]any)
	EmitDownloadMoved(ctx context.Context, payload map[string]any)
}

// PluginDownloads lists downloads from enabled download-client plugins.
type PluginDownloads interface {
	ListPluginDownloads(ctx context.Context) ([]pluginsdk.DownloadItem, error)
}

// SubtitleHook runs after a movie/episode video is imported (native subtitles).
type SubtitleHook interface {
	OnMovieImported(ctx context.Context, movieID int64)
	OnEpisodeImported(ctx context.Context, episodeID int64)
}

// Monitor polls built-in download clients (and plugins) and runs completed-download import.
type Monitor struct {
	store     *db.Store
	plugins   PluginDownloads
	notify    Notifier
	subtitles SubtitleHook
	interval  time.Duration
	mu        sync.Mutex
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

// SetSubtitleHook registers the post-import subtitle pipeline (optional).
func (m *Monitor) SetSubtitleHook(hook SubtitleHook) {
	m.subtitles = hook
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

// SyncProgress refreshes size/ETA from download clients without running imports.
// Safe to call from HTTP handlers (e.g. GET /api/v1/queue).
func (m *Monitor) SyncProgress(ctx context.Context) {
	m.syncFromClients(ctx, false)
}

func (m *Monitor) tick(ctx context.Context) {
	m.syncFromClients(ctx, true)
}

func (m *Monitor) syncFromClients(ctx context.Context, doImport bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appCfg, err := m.store.GetAppConfig()
	if err != nil {
		return
	}
	opts := appCfg.DownloadClientOptions
	cdh := opts.EnableCompletedDownloadHandling
	deleteData := opts.DeleteTorrentAndDataWhenComplete || opts.DeleteWhenSeedingComplete
	waitSeed := opts.DeleteWhenSeedingComplete

	queue, err := m.store.ListQueueItems("")
	if err != nil {
		return
	}
	active := make([]domain.QueueItem, 0, len(queue))
	for _, q := range queue {
		if q.DownloadID == "" {
			continue
		}
		if q.TrackedDownloadState == "imported" {
			// Keep imported items only while we still need to delete after seeding.
			if !waitSeed || !deleteData {
				continue
			}
		}
		active = append(active, q)
	}
	if len(active) == 0 {
		return
	}

	byID := map[string]trackedDownload{}
	clients, _ := m.store.ListDownloadClients()
	for _, dc := range clients {
		if !dc.Enable {
			continue
		}
		switch {
		case strings.EqualFold(dc.Implementation, "qBittorrent"):
			cli := qbittorrent.NewFromDownloadClient(dc)
			items, err := cli.List(ctx)
			if err != nil {
				log.Printf("downloadmonitor: qBittorrent list failed (%s): %v", dc.Name, err)
				continue
			}
			for _, it := range items {
				byID[it.Hash] = trackedDownload{
					ID:              it.Hash,
					Title:           it.Name,
					Completed:       qbittorrent.IsCompleted(it.State, it.Progress),
					SeedingComplete: qbittorrent.IsSeedingComplete(it.State, it.Progress),
					Progress:        it.Progress,
					Size:            it.Size,
					SizeLeft:        it.AmountLeft,
					Timeleft:        formatTimeleft(it.EtaSeconds, it.AmountLeft),
					EtaSeconds:      it.EtaSeconds,
					OutputPath:      firstNonEmpty(it.ContentPath, it.SavePath),
					ContentPath:     it.ContentPath,
					Client:          cli,
					RemoveDone:      dc.RemoveCompletedDownloads,
					ClientName:      dc.Name,
				}
			}
		case strings.EqualFold(dc.Implementation, "Transmission"):
			cli := transmission.NewFromDownloadClient(dc)
			items, err := cli.List(ctx)
			if err != nil {
				log.Printf("downloadmonitor: Transmission list failed (%s): %v", dc.Name, err)
				continue
			}
			for _, it := range items {
				content := firstNonEmpty(it.ContentPath, it.DownloadDir)
				byID[it.Hash] = trackedDownload{
					ID:              it.Hash,
					Title:           it.Name,
					Completed:       transmission.IsCompleted(it.Status, it.Progress, it.AmountLeft),
					SeedingComplete: transmission.IsSeedingComplete(it.Status, it.Progress, it.AmountLeft, it.IsFinished),
					Progress:        it.Progress,
					Size:            it.Size,
					SizeLeft:        it.AmountLeft,
					Timeleft:        formatTimeleft(it.EtaSeconds, it.AmountLeft),
					EtaSeconds:      it.EtaSeconds,
					OutputPath:      content,
					ContentPath:     content,
					Client:          cli,
					RemoveDone:      dc.RemoveCompletedDownloads,
					ClientName:      dc.Name,
				}
			}
		case strings.EqualFold(dc.Implementation, "Deluge"):
			cli := deluge.NewFromDownloadClient(dc)
			items, err := cli.List(ctx)
			if err != nil {
				log.Printf("downloadmonitor: Deluge list failed (%s): %v", dc.Name, err)
				continue
			}
			for _, it := range items {
				byID[it.Hash] = trackedDownload{
					ID:              it.Hash,
					Title:           it.Name,
					Completed:       deluge.IsCompleted(it.State, it.Progress),
					SeedingComplete: deluge.IsSeedingComplete(it.State, it.Progress),
					Progress:        it.Progress,
					Size:            it.Size,
					SizeLeft:        it.AmountLeft,
					Timeleft:        formatTimeleft(it.EtaSeconds, it.AmountLeft),
					EtaSeconds:      it.EtaSeconds,
					OutputPath:      firstNonEmpty(it.ContentPath, it.SavePath),
					ContentPath:     it.ContentPath,
					Client:          cli,
					RemoveDone:      dc.RemoveCompletedDownloads,
					ClientName:      dc.Name,
				}
			}
		case strings.EqualFold(dc.Implementation, "SABnzbd"):
			cli := sabnzbd.NewFromDownloadClient(dc)
			items, err := cli.List(ctx)
			if err != nil {
				log.Printf("downloadmonitor: SABnzbd list failed (%s): %v", dc.Name, err)
				continue
			}
			for _, it := range items {
				byID[it.ID] = trackedDownload{
					ID:              it.ID,
					Title:           it.Title,
					Completed:       it.Completed,
					SeedingComplete: it.Completed, // usenet has no seeding phase
					Progress:        it.Progress,
					Size:            it.Size,
					SizeLeft:        it.SizeLeft,
					Timeleft:        it.Timeleft,
					OutputPath:      it.OutputPath,
					ContentPath:     it.OutputPath,
					Client:          cli,
					RemoveDone:      dc.RemoveCompletedDownloads,
					ClientName:      dc.Name,
				}
			}
		case strings.EqualFold(dc.Implementation, "NZBGet"):
			cli := nzbget.NewFromDownloadClient(dc)
			items, err := cli.List(ctx)
			if err != nil {
				log.Printf("downloadmonitor: NZBGet list failed (%s): %v", dc.Name, err)
				continue
			}
			for _, it := range items {
				byID[it.ID] = trackedDownload{
					ID:              it.ID,
					Title:           it.Title,
					Completed:       it.Completed,
					SeedingComplete: it.Completed, // usenet has no seeding phase
					Progress:        it.Progress,
					Size:            it.Size,
					SizeLeft:        it.SizeLeft,
					OutputPath:      it.OutputPath,
					ContentPath:     it.OutputPath,
					Client:          cli,
					RemoveDone:      dc.RemoveCompletedDownloads,
					ClientName:      dc.Name,
				}
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

		// Deferred cleanup: already imported, waiting for seeding complete.
		if q.TrackedDownloadState == "imported" {
			if doImport {
				maybeRemoveFromClient(ctx, td, deleteData, waitSeed)
			}
			continue
		}

		q.Size = td.Size
		q.Sizeleft = td.SizeLeft
		q.Timeleft = td.Timeleft
		if td.EtaSeconds > 0 && td.EtaSeconds < 8640000 {
			q.EstimatedCompletion = time.Now().UTC().Add(time.Duration(td.EtaSeconds) * time.Second)
		} else {
			q.EstimatedCompletion = time.Time{}
		}
		srcPath := firstNonEmpty(td.ContentPath, td.OutputPath)
		mapped := applyRemotePathMapping(appCfg.RemotePathMappings, srcPath)
		if mapped != "" {
			q.OutputPath = mapped
		} else if srcPath != "" {
			q.OutputPath = srcPath
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
		q.Timeleft = "00:00:00"
		_ = m.store.UpdateQueueItem(q)

		if !doImport || !cdh {
			continue
		}
		if q.SeriesID == 0 && q.MovieID == 0 && q.AlbumID == 0 && q.ArtistID == 0 && q.EditionID == 0 && q.BookID == 0 {
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
		maybeRemoveFromClient(ctx, td, deleteData, waitSeed)
	}
}

type trackedDownload struct {
	ID              string
	Title           string
	Completed       bool
	SeedingComplete bool
	Progress        float64
	Size            int64
	SizeLeft        int64
	Timeleft        string
	EtaSeconds      int64
	OutputPath      string
	ContentPath     string
	Client          downloadRemover
	RemoveDone      bool
	ClientName      string
}

// maybeRemoveFromClient deletes the torrent from its download client after import
// when global delete options or per-client RemoveCompletedDownloads say so.
func maybeRemoveFromClient(ctx context.Context, td trackedDownload, deleteData, waitSeed bool) {
	if td.Client == nil {
		return
	}
	if deleteData {
		if waitSeed && !td.SeedingComplete {
			return
		}
		if err := td.Client.Remove(ctx, td.ID, true); err != nil {
			log.Printf("downloadmonitor: remove with data failed (%s / %s): %v", td.ClientName, td.ID, err)
		}
		return
	}
	if td.RemoveDone {
		if err := td.Client.Remove(ctx, td.ID, false); err != nil {
			log.Printf("downloadmonitor: remove failed (%s / %s): %v", td.ClientName, td.ID, err)
		}
	}
}

// formatTimeleft turns client ETA seconds into a Sonarr-style timeleft string.
// Returns empty when the client reports unknown/infinite ETA while data remains.
func formatTimeleft(etaSeconds, sizeLeft int64) string {
	if sizeLeft > 0 && (etaSeconds < 0 || etaSeconds >= 8640000) {
		return ""
	}
	if sizeLeft > 0 && etaSeconds == 0 {
		return ""
	}
	if etaSeconds < 0 {
		return ""
	}
	if etaSeconds >= 8640000 {
		return ""
	}
	days := etaSeconds / 86400
	hours := (etaSeconds % 86400) / 3600
	mins := (etaSeconds % 3600) / 60
	secs := etaSeconds % 60
	if days > 0 {
		return fmt.Sprintf("%d.%02d:%02d:%02d", days, hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}

func (m *Monitor) importCompleted(ctx context.Context, appCfg domain.AppConfig, q domain.QueueItem) error {
	srcRoot := q.OutputPath
	if srcRoot == "" {
		return fmt.Errorf("no output path")
	}

	var src string
	var err error
	switch strings.ToLower(q.MediaType) {
	case "music":
		src, err = findPrimaryMedia(srcRoot, isAudio)
	case "ebook":
		src, err = findPrimaryMedia(srcRoot, isEbook)
	case "audiobook":
		src, err = findPrimaryMedia(srcRoot, isAudiobook)
	default:
		src, err = findPrimaryVideo(srcRoot)
	}
	if err != nil {
		return err
	}
	dest, err := m.destinationPath(appCfg, q, src, filepath.Ext(src))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	hardlink := appCfg.MediaManagement.CopyUsingHardlinks
	if err := importFile(src, dest, hardlink); err != nil {
		return err
	}
	if appCfg.MediaManagement.ImportExtraFiles {
		exts := mediafile.ParseExtensions(appCfg.MediaManagement.ExtraFileExtensions)
		if len(exts) == 0 {
			exts = mediafile.ParseExtensions("srt")
		}
		extras, err := mediafile.ListExtraFiles(srcRoot, src, exts)
		if err != nil {
			log.Printf("downloadmonitor: list extras: %v", err)
		} else {
			for _, extra := range extras {
				extraDest := mediafile.ExtraDestPath(dest, extra)
				if err := os.MkdirAll(filepath.Dir(extraDest), 0o755); err != nil {
					log.Printf("downloadmonitor: extra mkdir %s: %v", extraDest, err)
					continue
				}
				if err := importFile(extra, extraDest, hardlink); err != nil {
					log.Printf("downloadmonitor: extra import %s: %v", extra, err)
				}
			}
		}
	}

	if appCfg.MediaManagement.DeleteUnwantedFiles {
		unwanted := mediafile.UnwantedExtensions(appCfg.MediaManagement, q.MediaType)
		if _, err := mediafile.DeleteUnwantedInDir(srcRoot, unwanted, appCfg.MediaManagement.RecycleBin); err != nil {
			log.Printf("downloadmonitor: unwanted cleanup: %v", err)
		}
	}

	sceneName := strings.TrimSpace(q.Title)
	releaseGroup := parseReleaseGroup(sceneName)
	var size int64
	if fi, err := os.Stat(dest); err == nil {
		size = fi.Size()
	}

	if q.MovieID > 0 {
		if _, err := m.store.UpsertMovieFile(q.MovieID, dest, sceneName, releaseGroup, size); err != nil {
			log.Printf("downloadmonitor: movie file record failed: %v", err)
			if mv, err := m.store.GetMovieByID(q.MovieID); err == nil && mv != nil {
				mv.HasFile = true
				mv.Path = filepath.Dir(dest)
				_, _ = m.store.UpdateMovie(*mv)
			} else {
				_ = m.store.MarkMovieHasFile(q.MovieID, true)
			}
		} else if m.subtitles != nil {
			m.subtitles.OnMovieImported(ctx, q.MovieID)
		}
	}
	if q.SeriesID > 0 {
		episodeID := q.EpisodeID
		if episodeID == 0 {
			if season, ep, ok := parseSeasonEpisode(q.Title); ok {
				if eps, err := m.store.ListEpisodes(q.SeriesID); err == nil {
					for _, e := range eps {
						if e.SeasonNumber == season && e.EpisodeNumber == ep {
							episodeID = e.ID
							break
						}
					}
				}
				if episodeID == 0 {
					_ = m.store.MarkEpisodeHasFileByNumbers(q.SeriesID, season, ep, true)
				}
			}
		}
		if episodeID > 0 {
			if _, err := m.store.UpsertEpisodeFile(q.SeriesID, episodeID, dest, sceneName, releaseGroup, size); err != nil {
				log.Printf("downloadmonitor: episode file record failed: %v", err)
				_ = m.store.MarkEpisodeHasFile(episodeID, true)
			} else if m.subtitles != nil {
				m.subtitles.OnEpisodeImported(ctx, episodeID)
			}
		}
	}
	if q.AlbumID > 0 {
		_ = m.store.MarkAlbumTracksHasFile(q.AlbumID, true)
	} else if q.ArtistID > 0 && q.AlbumID == 0 {
		// Best-effort: mark no specific album; import still lands under artist path.
	}
	if q.EditionID > 0 {
		_ = m.store.MarkEditionHasFile(q.EditionID, true)
		if ed, err := m.store.GetEditionByID(q.EditionID); err == nil && ed != nil {
			ed.HasFile = true
			ed.Path = dest
			_, _ = m.store.UpdateEdition(*ed)
		}
	}

	_ = m.store.CreateHistory(domain.HistoryRecord{
		MediaType:   q.MediaType,
		SeriesID:    q.SeriesID,
		EpisodeID:   q.EpisodeID,
		MovieID:     q.MovieID,
		ArtistID:    q.ArtistID,
		AlbumID:     q.AlbumID,
		AuthorID:    q.AuthorID,
		BookID:      q.BookID,
		EditionID:   q.EditionID,
		SourceTitle: q.Title,
		Quality:     map[string]any{"quality": map[string]any{"name": "Unknown"}},
		EventType:   "downloadFolderImported",
		Data: map[string]any{
			"sourcePath": src,
			"destPath":   dest,
			"downloadId": q.DownloadID,
		},
	})

	if m.notify != nil {
		completePayload := importNotifyPayload(q, dest)
		m.notify.EmitDownloadComplete(ctx, completePayload)
		movedPayload := importNotifyPayload(q, dest)
		movedPayload["sourcePath"] = src
		movedPayload["destPath"] = dest
		delete(movedPayload, "path")
		m.notify.EmitDownloadMoved(ctx, movedPayload)
	}
	return nil
}

// importNotifyPayload builds the common lifecycle fields for import notifications.
func importNotifyPayload(q domain.QueueItem, dest string) map[string]any {
	payload := map[string]any{
		"title":     q.Title,
		"queueId":   q.ID,
		"path":      dest,
		"mediaType": q.MediaType,
	}
	if q.SeriesID > 0 {
		payload["seriesId"] = q.SeriesID
	}
	if q.EpisodeID > 0 {
		payload["episodeId"] = q.EpisodeID
	}
	if q.MovieID > 0 {
		payload["movieId"] = q.MovieID
	}
	if q.ArtistID > 0 {
		payload["artistId"] = q.ArtistID
	}
	if q.AlbumID > 0 {
		payload["albumId"] = q.AlbumID
	}
	if q.AuthorID > 0 {
		payload["authorId"] = q.AuthorID
	}
	if q.BookID > 0 {
		payload["bookId"] = q.BookID
	}
	if q.EditionID > 0 {
		payload["editionId"] = q.EditionID
	}
	return payload
}

func (m *Monitor) destinationPath(appCfg domain.AppConfig, q domain.QueueItem, src, ext string) (string, error) {
	if ext == "" {
		ext = ".mkv"
	}
	if q.MovieID > 0 {
		mv, err := m.store.GetMovieByID(q.MovieID)
		if err != nil || mv == nil {
			return "", fmt.Errorf("movie %d not found", q.MovieID)
		}
		mm := appCfg.MediaManagement
		base := strings.TrimSpace(mv.Path)
		if base == "" {
			root := firstRoot(appCfg, m.store, "movie")
			folderFmt := strings.TrimSpace(mm.MovieFolderFormat)
			if folderFmt == "" {
				folderFmt = "{Movie Title} ({Release Year})"
			}
			folder := applyMovieFormat(folderFmt, mv.Title, mv.Year, "")
			folder = cleanFileName(folder, mm)
			if folder == "" {
				folder = cleanFileName(mv.Title, mm)
			}
			base = filepath.Join(root, folder)
		}
		var file string
		if mm.RenameMovies {
			fileFmt := strings.TrimSpace(mm.MovieFileFormat)
			if fileFmt == "" {
				fileFmt = "{Movie Title} ({Release Year})"
			}
			file = applyMovieFormat(fileFmt, mv.Title, mv.Year, "")
			file = cleanFileName(file, mm)
		} else {
			// Keep the release basename (Radarr behavior when renameMovies is off).
			file = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
			if file == "" || file == "." {
				file = strings.TrimSuffix(filepath.Base(q.Title), filepath.Ext(q.Title))
			}
			file = cleanFileName(file, mm)
		}
		if file == "" {
			file = cleanFileName(mv.Title, mm)
			if mv.Year > 0 {
				file = fmt.Sprintf("%s (%d)", file, mv.Year)
			}
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
	if q.AlbumID > 0 || q.ArtistID > 0 {
		artistID := q.ArtistID
		albumTitle := sanitizeName(q.Title)
		if q.AlbumID > 0 {
			if al, err := m.store.GetAlbumByID(q.AlbumID); err == nil && al != nil {
				artistID = al.ArtistID
				albumTitle = sanitizeName(al.Title)
			}
		}
		ar, err := m.store.GetArtistByID(artistID)
		if err != nil || ar == nil {
			return "", fmt.Errorf("artist %d not found", artistID)
		}
		base := strings.TrimSpace(ar.Path)
		if base == "" {
			root := firstRoot(appCfg, m.store, "music")
			base = filepath.Join(root, sanitizeName(ar.Title))
		}
		return filepath.Join(base, albumTitle, sanitizeName(q.Title)+ext), nil
	}
	if q.EditionID > 0 || q.BookID > 0 {
		var ed *domain.Edition
		if q.EditionID > 0 {
			ed, _ = m.store.GetEditionByID(q.EditionID)
		}
		title := sanitizeName(q.Title)
		rootType := "ebook"
		base := ""
		if ed != nil {
			title = sanitizeName(ed.Title)
			if ed.Format == "audiobook" {
				rootType = "audiobook"
			}
			if ed.Path != "" {
				base = strings.TrimSpace(filepath.Dir(ed.Path))
			}
			if (base == "" || base == ".") && ed.RootFolderPath != "" {
				base = filepath.Join(ed.RootFolderPath, sanitizeName(ed.Title))
			}
		}
		if base == "" {
			authorID := q.AuthorID
			if ed != nil {
				authorID = ed.AuthorID
			}
			if authorID > 0 {
				if au, err := m.store.GetAuthorByID(authorID); err == nil && au != nil {
					base = filepath.Join(strings.TrimSpace(au.Path), title)
					if au.Path == "" {
						base = filepath.Join(firstRoot(appCfg, m.store, rootType), sanitizeName(au.Title), title)
					}
				}
			}
		}
		if base == "" {
			base = filepath.Join(firstRoot(appCfg, m.store, rootType), title)
		}
		return filepath.Join(base, title+ext), nil
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

func isVideo(path string) bool    { return mediafile.IsVideo(path) }
func isAudio(path string) bool    { return mediafile.IsAudio(path) }
func isEbook(path string) bool    { return mediafile.IsEbook(path) }
func isAudiobook(path string) bool { return mediafile.IsAudiobook(path) }

func findPrimaryMedia(root string, match func(string) bool) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		if match(root) {
			return root, nil
		}
		return "", fmt.Errorf("unsupported media file: %s", root)
	}
	var best string
	var bestSize int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !match(path) {
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
		return "", fmt.Errorf("no matching media files under %s", root)
	}
	return best, nil
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

// applyMovieFormat replaces common Radarr movie naming tokens.
func applyMovieFormat(format, title string, year int, quality string) string {
	yearStr := ""
	if year > 0 {
		yearStr = fmt.Sprintf("%d", year)
	}
	replacer := strings.NewReplacer(
		"{Movie Title}", title,
		"{Movie CleanTitle}", sanitizeName(title),
		"{Release Year}", yearStr,
		"{Quality Full}", quality,
		"{Quality Title}", quality,
	)
	out := replacer.Replace(format)
	out = multiSpacePattern.ReplaceAllString(out, " ")
	return strings.TrimSpace(strings.Trim(out, " .-_"))
}

// cleanFileName applies illegal-character / colon replacement from media management settings.
func cleanFileName(name string, mm domain.MediaManagementConfig) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !mm.ReplaceIllegalCharacters {
		return sanitizeName(name)
	}
	colon := strings.TrimSpace(mm.ColonReplacementFormat)
	switch strings.ToLower(colon) {
	case "delete":
		name = strings.ReplaceAll(name, ":", "")
	case "dash":
		name = strings.ReplaceAll(name, ":", "-")
	case "spaceDash":
		name = strings.ReplaceAll(name, ":", " -")
	case "spaceDashSpace":
		name = strings.ReplaceAll(name, ":", " - ")
	case "custom":
		custom := mm.CustomColonReplacementFormat
		name = strings.ReplaceAll(name, ":", custom)
	default: // smart
		name = strings.ReplaceAll(name, ": ", " - ")
		name = strings.ReplaceAll(name, ":", " -")
	}
	replacer := strings.NewReplacer(`/`, "-", `\`, "-", `*`, "", `?`, "", `"`, "", `<`, "", `>`, "", `|`, "")
	return strings.TrimSpace(replacer.Replace(name))
}

var (
	releaseGroupPattern = regexp.MustCompile(`(?i)[-.]([A-Za-z0-9]+)(?:\[[^\]]*\])?(?:\.(?:mkv|mp4|avi))?$`)
	multiSpacePattern   = regexp.MustCompile(`\s{2,}`)
)

func parseReleaseGroup(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	base := filepath.Base(title)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	m := releaseGroupPattern.FindStringSubmatch(base)
	if len(m) < 2 {
		return ""
	}
	return m[1]
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
