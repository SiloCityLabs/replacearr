package libraryimport

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediafile"
)

type ScanItem struct {
	Path          string `json:"path"`
	RelativePath  string `json:"relativePath"`
	Size          int64  `json:"size"`
	GuessedTitle  string `json:"guessedTitle"`
	SeasonNumber  int    `json:"seasonNumber,omitempty"`
	EpisodeNumber int    `json:"episodeNumber,omitempty"`
}

type ScanResult struct {
	Items []ScanItem `json:"items"`
}

type ImportItem struct {
	Path          string `json:"path"`
	MatchID       int64  `json:"matchId"`
	SeasonNumber  int    `json:"seasonNumber,omitempty"`
	EpisodeNumber int    `json:"episodeNumber,omitempty"`
}

var seasonEpRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,3})`)

func Scan(store *db.Store, mediaType, root string) (*ScanResult, error) {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	match := mediafile.MatcherForMediaType(mediaType)
	known := knownPaths(store, mediaType)

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !match(root) {
			return &ScanResult{Items: []ScanItem{}}, nil
		}
		if known[filepath.Clean(root)] {
			return &ScanResult{Items: []ScanItem{}}, nil
		}
		fi, _ := os.Stat(root)
		item := scanItemFromPath(root, root, fi.Size())
		return &ScanResult{Items: []ScanItem{item}}, nil
	}

	items := []ScanItem{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !match(path) {
			return nil
		}
		clean := filepath.Clean(path)
		if known[clean] {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		items = append(items, scanItemFromPath(path, rel, fi.Size()))
		return nil
	})
	return &ScanResult{Items: items}, nil
}

func scanItemFromPath(path, rel string, size int64) ScanItem {
	base := filepath.Base(path)
	title := strings.TrimSuffix(base, filepath.Ext(base))
	item := ScanItem{
		Path:         path,
		RelativePath: rel,
		Size:         size,
		GuessedTitle: title,
	}
	if m := seasonEpRe.FindStringSubmatch(base); len(m) == 3 {
		fmt.Sscanf(m[1], "%d", &item.SeasonNumber)
		fmt.Sscanf(m[2], "%d", &item.EpisodeNumber)
		cleaned := seasonEpRe.ReplaceAllString(title, "")
		cleaned = strings.ReplaceAll(cleaned, ".", " ")
		cleaned = strings.ReplaceAll(cleaned, "_", " ")
		item.GuessedTitle = strings.TrimSpace(cleaned)
	}
	return item
}

func knownPaths(store *db.Store, mediaType string) map[string]bool {
	out := map[string]bool{}
	switch mediaType {
	case "tv", "series":
		files, err := store.ListEpisodeFiles(0)
		if err == nil {
			for _, f := range files {
				if f.Path != "" {
					out[filepath.Clean(f.Path)] = true
				}
			}
		}
	case "movies", "movie":
		files, err := store.ListMovieFiles(0)
		if err == nil {
			for _, f := range files {
				if f.Path != "" {
					out[filepath.Clean(f.Path)] = true
				}
			}
		}
	}
	return out
}

// ImportSelected copies/hardlinks files into library and updates DB.
func ImportSelected(store *db.Store, appCfg domain.AppConfig, mediaType string, items []ImportItem) (int, error) {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	imported := 0
	for _, it := range items {
		if it.MatchID <= 0 || strings.TrimSpace(it.Path) == "" {
			continue
		}
		src := it.Path
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		dest, episodeID, err := destinationFor(store, appCfg, mediaType, it, filepath.Ext(src))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			continue
		}
		if err := copyOrLink(src, dest, appCfg.MediaManagement.CopyUsingHardlinks); err != nil {
			continue
		}
		size := fi.Size()
		switch mediaType {
		case "tv", "series":
			if episodeID > 0 {
				_, _ = store.UpsertEpisodeFile(it.MatchID, episodeID, dest, filepath.Base(src), "", size)
			} else if it.SeasonNumber > 0 && it.EpisodeNumber > 0 {
				_ = store.MarkEpisodeHasFileByNumbers(it.MatchID, it.SeasonNumber, it.EpisodeNumber, true)
			}
			_ = store.CreateHistory(domain.HistoryRecord{
				MediaType: "tv", SeriesID: it.MatchID, EpisodeID: episodeID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": dest},
			})
		case "movies", "movie":
			_, _ = store.UpsertMovieFile(it.MatchID, dest, filepath.Base(src), "", size)
			_ = store.CreateHistory(domain.HistoryRecord{
				MediaType: "movie", MovieID: it.MatchID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": dest},
			})
		case "music":
			_ = store.CreateHistory(domain.HistoryRecord{
				MediaType: "music", ArtistID: it.MatchID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": dest},
			})
		case "books", "ebook", "audiobook":
			_ = store.CreateHistory(domain.HistoryRecord{
				MediaType: "ebook", BookID: it.MatchID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": dest},
			})
		}
		if appCfg.MediaManagement.DeleteUnwantedFiles {
			unwanted := mediafile.UnwantedExtensions(appCfg.MediaManagement, mediaType)
			_, _ = mediafile.DeleteUnwantedInDir(filepath.Dir(src), unwanted, appCfg.MediaManagement.RecycleBin)
		}
		imported++
	}
	return imported, nil
}

func destinationFor(store *db.Store, appCfg domain.AppConfig, mediaType string, it ImportItem, ext string) (string, int64, error) {
	switch mediaType {
	case "tv", "series":
		sr, err := store.GetSeriesByID(it.MatchID)
		if err != nil || sr == nil {
			return "", 0, fmt.Errorf("series not found")
		}
		base := strings.TrimSpace(sr.Path)
		if base == "" {
			base = filepath.Join(sr.RootFolderPath, sanitize(sr.Title))
		}
		season, ep := it.SeasonNumber, it.EpisodeNumber
		if season == 0 {
			season = 1
		}
		if ep == 0 {
			ep = 1
		}
		var episodeID int64
		if eps, err := store.ListEpisodes(sr.ID); err == nil {
			for _, e := range eps {
				if e.SeasonNumber == season && e.EpisodeNumber == ep {
					episodeID = e.ID
					break
				}
			}
		}
		file := fmt.Sprintf("%s - S%02dE%02d%s", sanitize(sr.Title), season, ep, ext)
		return filepath.Join(base, fmt.Sprintf("Season %02d", season), file), episodeID, nil
	case "movies", "movie":
		mv, err := store.GetMovieByID(it.MatchID)
		if err != nil || mv == nil {
			return "", 0, fmt.Errorf("movie not found")
		}
		base := strings.TrimSpace(mv.Path)
		if base == "" {
			base = filepath.Join(mv.RootFolderPath, sanitize(mv.Title))
		}
		return filepath.Join(base, sanitize(mv.Title)+ext), 0, nil
	case "music":
		ar, err := store.GetArtistByID(it.MatchID)
		if err != nil || ar == nil {
			return "", 0, fmt.Errorf("artist not found")
		}
		base := strings.TrimSpace(ar.Path)
		if base == "" {
			base = filepath.Join(ar.RootFolderPath, sanitize(ar.Title))
		}
		return filepath.Join(base, filepath.Base(it.Path)), 0, nil
	case "books", "ebook", "audiobook":
		bk, err := store.GetBookByID(it.MatchID)
		if err != nil || bk == nil {
			return "", 0, fmt.Errorf("book not found")
		}
		au, _ := store.GetAuthorByID(bk.AuthorID)
		base := ""
		if au != nil {
			base = strings.TrimSpace(au.Path)
			if base == "" {
				base = filepath.Join(au.RootFolderPath, sanitize(au.Title))
			}
		}
		if base == "" {
			base = filepath.Join(".", "media", "books", sanitize(bk.Title))
		}
		_ = appCfg
		return filepath.Join(base, sanitize(bk.Title)+ext), 0, nil
	default:
		return "", 0, fmt.Errorf("unsupported media type")
	}
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", " -", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	return strings.TrimSpace(repl.Replace(s))
}

func copyOrLink(src, dest string, hardlink bool) error {
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
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}
