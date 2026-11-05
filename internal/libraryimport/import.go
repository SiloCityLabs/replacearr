package libraryimport

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"github.com/replacearr/replacearr/internal/mediafile"
	"github.com/replacearr/replacearr/internal/metadata"
)

type MatchCandidate struct {
	LibraryID     int64  `json:"libraryId,omitempty"`
	TmdbID        int    `json:"tmdbId,omitempty"`
	TvdbID        int    `json:"tvdbId,omitempty"`
	ImdbID        string `json:"imdbId,omitempty"`
	ForeignID     string `json:"foreignId,omitempty"` // MusicBrainz MBID or OpenLibrary work key
	AuthorName    string `json:"authorName,omitempty"`
	Format        string `json:"format,omitempty"` // ebook | audiobook
	Title         string `json:"title"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
	InLibrary     bool   `json:"inLibrary"`
}

type ScanItem struct {
	Path          string           `json:"path"`
	RelativePath  string           `json:"relativePath"`
	Size          int64            `json:"size"`
	GuessedTitle  string           `json:"guessedTitle"`
	GuessedYear   int              `json:"guessedYear,omitempty"`
	SeasonNumber  int              `json:"seasonNumber,omitempty"`
	EpisodeNumber int              `json:"episodeNumber,omitempty"`
	Matches       []MatchCandidate `json:"matches"`
	Match         *MatchCandidate  `json:"match,omitempty"`
}

type ScanResult struct {
	Items []ScanItem `json:"items"`
}

type ImportItem struct {
	Path             string `json:"path"`
	MatchID          int64  `json:"matchId,omitempty"`
	TmdbID           int    `json:"tmdbId,omitempty"`
	TvdbID           int    `json:"tvdbId,omitempty"`
	ForeignID        string `json:"foreignId,omitempty"`
	Format           string `json:"format,omitempty"`
	QualityProfileID int64  `json:"qualityProfileId,omitempty"`
	SeasonNumber     int    `json:"seasonNumber,omitempty"`
	EpisodeNumber    int    `json:"episodeNumber,omitempty"`
}

type ScanOptions struct {
	Store            *db.Store
	Meta             *metadata.Service
	MediaType        string
	Root             string
	Language         string
	MaxMatches       int
	MatchConcurrency int
}

type ImportOptions struct {
	Store              *db.Store
	Meta               *metadata.Service
	AppCfg             domain.AppConfig
	MediaType          string
	RootFolderPath     string
	QualityProfileID   int64
	Language           string
	SyncSeriesEpisodes func(ctx context.Context, sr *domain.Series) error
	SyncArtistAlbums   func(ctx context.Context, artist *domain.Artist) error
}

var seasonEpRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,3})`)

func Scan(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	mediaType := strings.ToLower(strings.TrimSpace(opts.MediaType))
	root := opts.Root
	match := mediafile.MatcherForMediaType(mediaType)
	known := knownPaths(opts.Store, mediaType)
	maxMatches := opts.MaxMatches
	if maxMatches <= 0 {
		maxMatches = 8
	}
	workers := opts.MatchConcurrency
	if workers <= 0 {
		workers = 4
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	raw := []ScanItem{}
	appendFile := func(path string, size int64) {
		clean := filepath.Clean(path)
		if known[clean] {
			return
		}
		rel, _ := filepath.Rel(root, path)
		item := scanItemFromPath(root, path, rel, size)
		raw = append(raw, item)
	}

	if !info.IsDir() {
		if !match(root) {
			return &ScanResult{Items: []ScanItem{}}, nil
		}
		fi, _ := os.Stat(root)
		appendFile(root, fi.Size())
	} else {
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
			appendFile(path, fi.Size())
			return nil
		})
	}

	if opts.Meta != nil {
		switch mediaType {
		case "movies", "movie", "tv", "series", "music", "books", "ebook", "audiobook":
			if mediaType == "music" && workers > 1 {
				// MusicBrainz is rate-limited (~1 req/s).
				workers = 1
			}
			enrichMatches(ctx, opts, mediaType, raw, maxMatches, workers)
		}
	}

	return &ScanResult{Items: raw}, nil
}

func scanItemFromPath(scanRoot, path, rel string, size int64) ScanItem {
	parsed := guessNameForFile(scanRoot, path)
	item := ScanItem{
		Path:         path,
		RelativePath: rel,
		Size:         size,
		GuessedTitle: parsed.Title,
		GuessedYear:  parsed.Year,
		Matches:      []MatchCandidate{},
	}
	base := filepath.Base(path)
	if m := seasonEpRe.FindStringSubmatch(base); len(m) == 3 {
		fmt.Sscanf(m[1], "%d", &item.SeasonNumber)
		fmt.Sscanf(m[2], "%d", &item.EpisodeNumber)
		if item.GuessedTitle == "" {
			cleaned := seasonEpRe.ReplaceAllString(strings.TrimSuffix(base, filepath.Ext(base)), "")
			item.GuessedTitle = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(cleaned, ".", " "), "_", " ")), " "))
		}
	}
	return item
}

func enrichMatches(ctx context.Context, opts ScanOptions, mediaType string, items []ScanItem, maxMatches, workers int) {
	type job struct{ idx int }
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex

	libraryMovies := map[int]int64{}
	librarySeries := map[int]int64{}
	libraryArtists := map[string]int64{}
	libraryBooks := map[string]int64{}
	if mediaType == "movies" || mediaType == "movie" {
		if all, err := opts.Store.ListMovies(); err == nil {
			for _, m := range all {
				if m.TmdbID > 0 {
					libraryMovies[m.TmdbID] = m.ID
				}
			}
		}
	}
	if mediaType == "tv" || mediaType == "series" {
		if all, err := opts.Store.ListSeries(); err == nil {
			for _, s := range all {
				if s.TvdbID > 0 {
					librarySeries[s.TvdbID] = s.ID
				}
			}
		}
	}
	if mediaType == "music" {
		if all, err := opts.Store.ListArtists(); err == nil {
			for _, a := range all {
				if a.ForeignArtistID != "" {
					libraryArtists[a.ForeignArtistID] = a.ID
				}
			}
		}
	}
	if mediaType == "books" || mediaType == "ebook" || mediaType == "audiobook" {
		if all, err := opts.Store.ListBooks(0); err == nil {
			for _, b := range all {
				if b.ForeignBookID != "" {
					libraryBooks[b.ForeignBookID] = b.ID
				}
			}
		}
	}

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ctx.Err() != nil {
				return
			}
			item := items[j.idx]
			parsed := ParsedName{Title: item.GuessedTitle, Year: item.GuessedYear}
			// Re-parse path for ids embedded in folder/file names.
			reparsed := guessNameForFile(opts.Root, item.Path)
			if reparsed.TmdbID > 0 {
				parsed.TmdbID = reparsed.TmdbID
			}
			if reparsed.ImdbID != "" {
				parsed.ImdbID = reparsed.ImdbID
			}
			term := lookupTerm(parsed)
			if term == "" {
				continue
			}

			var matches []MatchCandidate
			switch mediaType {
			case "movies", "movie":
				movies, err := opts.Meta.SearchMovies(ctx, term)
				if err != nil {
					continue
				}
				matches = movieCandidates(movies, parsed.Year, libraryMovies, maxMatches)
			case "tv", "series":
				series, err := opts.Meta.SearchSeries(ctx, term, opts.Language)
				if err != nil {
					continue
				}
				matches = seriesCandidates(series, parsed.Year, librarySeries, maxMatches)
			case "music":
				artists, err := opts.Meta.SearchArtists(ctx, term)
				if err != nil {
					continue
				}
				matches = artistCandidates(artists, libraryArtists, maxMatches)
			case "books", "ebook", "audiobook":
				books, err := opts.Meta.SearchBooks(ctx, term)
				if err != nil {
					continue
				}
				format := detectBookFormat(item.Path, mediaType)
				matches = bookCandidates(books, libraryBooks, format, maxMatches)
			}
			if len(matches) == 0 {
				continue
			}
			mu.Lock()
			items[j.idx].Matches = matches
			sel := matches[0]
			items[j.idx].Match = &sel
			mu.Unlock()
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	for i := range items {
		jobs <- job{idx: i}
	}
	close(jobs)
	wg.Wait()
}

func movieCandidates(movies []domain.Movie, year int, library map[int]int64, max int) []MatchCandidate {
	scored := make([]MatchCandidate, 0, len(movies))
	for _, m := range movies {
		if m.TmdbID <= 0 || m.Title == "" {
			continue
		}
		c := MatchCandidate{
			TmdbID:    m.TmdbID,
			ImdbID:    m.ImdbID,
			Title:     m.Title,
			Year:      m.Year,
			Overview:  m.Overview,
			LibraryID: library[m.TmdbID],
			InLibrary: library[m.TmdbID] > 0,
		}
		scored = append(scored, c)
	}
	if year > 0 {
		exact := make([]MatchCandidate, 0, len(scored))
		rest := make([]MatchCandidate, 0, len(scored))
		for _, c := range scored {
			if c.Year == year {
				exact = append(exact, c)
			} else {
				rest = append(rest, c)
			}
		}
		scored = append(exact, rest...)
	}
	if len(scored) > max {
		scored = scored[:max]
	}
	return scored
}

func seriesCandidates(series []domain.Series, year int, library map[int]int64, max int) []MatchCandidate {
	scored := make([]MatchCandidate, 0, len(series))
	for _, s := range series {
		if s.TvdbID <= 0 || s.Title == "" {
			continue
		}
		c := MatchCandidate{
			TvdbID:    s.TvdbID,
			ImdbID:    s.ImdbID,
			Title:     s.Title,
			Year:      s.Year,
			Overview:  s.Overview,
			LibraryID: library[s.TvdbID],
			InLibrary: library[s.TvdbID] > 0,
		}
		scored = append(scored, c)
	}
	if year > 0 {
		exact := make([]MatchCandidate, 0, len(scored))
		rest := make([]MatchCandidate, 0, len(scored))
		for _, c := range scored {
			if c.Year == year {
				exact = append(exact, c)
			} else {
				rest = append(rest, c)
			}
		}
		scored = append(exact, rest...)
	}
	if len(scored) > max {
		scored = scored[:max]
	}
	return scored
}

func artistCandidates(artists []domain.Artist, library map[string]int64, max int) []MatchCandidate {
	out := make([]MatchCandidate, 0, len(artists))
	for _, a := range artists {
		if a.Title == "" || a.ForeignArtistID == "" {
			continue
		}
		out = append(out, MatchCandidate{
			ForeignID:      a.ForeignArtistID,
			Title:          a.Title,
			Overview:       a.Overview,
			Disambiguation: a.Disambiguation,
			LibraryID:      library[a.ForeignArtistID],
			InLibrary:      library[a.ForeignArtistID] > 0,
		})
		if len(out) >= max {
			break
		}
	}
	return out
}

func bookCandidates(books []domain.Book, library map[string]int64, format string, max int) []MatchCandidate {
	out := make([]MatchCandidate, 0, len(books))
	for _, b := range books {
		if b.Title == "" || b.ForeignBookID == "" {
			continue
		}
		year := 0
		if y, err := strconv.Atoi(strings.TrimSpace(b.ReleaseDate)); err == nil {
			year = y
		} else if len(b.ReleaseDate) >= 4 {
			if y, err := strconv.Atoi(b.ReleaseDate[:4]); err == nil {
				year = y
			}
		}
		authorName := strings.TrimPrefix(b.Overview, "by ")
		if authorName == b.Overview {
			authorName = ""
		}
		out = append(out, MatchCandidate{
			ForeignID:  b.ForeignBookID,
			Title:      b.Title,
			Year:       year,
			Overview:   b.Overview,
			AuthorName: authorName,
			Format:     format,
			LibraryID:  library[b.ForeignBookID],
			InLibrary:  library[b.ForeignBookID] > 0,
		})
		if len(out) >= max {
			break
		}
	}
	return out
}

func detectBookFormat(path, mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "audiobook" || mediafile.IsAudiobook(path) {
		return "audiobook"
	}
	return "ebook"
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

// ImportSelected adds missing library rows from metadata when needed, then
// registers existing on-disk files in place (no copy/move for library import).
func ImportSelected(ctx context.Context, opts ImportOptions, items []ImportItem) (int, error) {
	if opts.Store == nil {
		return 0, fmt.Errorf("store is required")
	}
	mediaType := strings.ToLower(strings.TrimSpace(opts.MediaType))
	imported := 0
	for _, it := range items {
		if strings.TrimSpace(it.Path) == "" {
			continue
		}
		src := it.Path
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		profileID := it.QualityProfileID
		if profileID <= 0 {
			profileID = opts.QualityProfileID
		}
		if profileID <= 0 {
			profileID = 1
		}

		switch mediaType {
		case "movies", "movie":
			mv, err := resolveMovie(ctx, opts, it, profileID, src)
			if err != nil || mv == nil {
				continue
			}
			if _, err := opts.Store.UpsertMovieFile(mv.ID, src, filepath.Base(src), "", fi.Size()); err != nil {
				continue
			}
			_ = opts.Store.CreateHistory(domain.HistoryRecord{
				MediaType: "movie", MovieID: mv.ID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": src, "inPlace": true},
			})
			imported++
		case "tv", "series":
			sr, err := resolveSeries(ctx, opts, it, profileID, src)
			if err != nil || sr == nil {
				continue
			}
			season, ep := it.SeasonNumber, it.EpisodeNumber
			if season == 0 || ep == 0 {
				base := filepath.Base(src)
				if m := seasonEpRe.FindStringSubmatch(base); len(m) == 3 {
					fmt.Sscanf(m[1], "%d", &season)
					fmt.Sscanf(m[2], "%d", &ep)
				}
			}
			var episodeID int64
			if season > 0 && ep > 0 {
				if eps, err := opts.Store.ListEpisodes(sr.ID); err == nil {
					for _, e := range eps {
						if e.SeasonNumber == season && e.EpisodeNumber == ep {
							episodeID = e.ID
							break
						}
					}
				}
			}
			if episodeID > 0 {
				_, _ = opts.Store.UpsertEpisodeFile(sr.ID, episodeID, src, filepath.Base(src), "", fi.Size())
			} else if season > 0 && ep > 0 {
				_ = opts.Store.MarkEpisodeHasFileByNumbers(sr.ID, season, ep, true)
			}
			_ = opts.Store.CreateHistory(domain.HistoryRecord{
				MediaType: "tv", SeriesID: sr.ID, EpisodeID: episodeID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": src, "inPlace": true},
			})
			imported++
		case "music":
			ar, err := resolveArtist(ctx, opts, it, profileID, src)
			if err != nil || ar == nil {
				continue
			}
			_ = opts.Store.CreateHistory(domain.HistoryRecord{
				MediaType: "music", ArtistID: ar.ID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": src, "inPlace": true},
			})
			imported++
		case "books", "ebook", "audiobook":
			bk, err := resolveBook(ctx, opts, it, profileID, src, mediaType)
			if err != nil || bk == nil {
				continue
			}
			_ = opts.Store.CreateHistory(domain.HistoryRecord{
				MediaType: "ebook", BookID: bk.ID,
				SourceTitle: filepath.Base(src), EventType: "downloadFolderImported",
				Quality: map[string]any{"quality": map[string]any{"name": "Unknown"}},
				Data:    map[string]any{"sourcePath": src, "destPath": src, "inPlace": true},
			})
			imported++
		}
	}
	return imported, nil
}

func resolveMovie(ctx context.Context, opts ImportOptions, it ImportItem, profileID int64, src string) (*domain.Movie, error) {
	if it.MatchID > 0 {
		return opts.Store.GetMovieByID(it.MatchID)
	}
	tmdbID := it.TmdbID
	if tmdbID > 0 {
		if existing, err := opts.Store.GetMovieByTmdbID(tmdbID); err == nil && existing != nil {
			return existing, nil
		}
	}
	if tmdbID <= 0 || opts.Meta == nil {
		return nil, fmt.Errorf("movie match required")
	}
	meta, err := opts.Meta.GetMovieByTMDB(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	folder := filepath.Dir(src)
	root := strings.TrimSpace(opts.RootFolderPath)
	if root == "" {
		root = folder
	}
	meta.QualityProfileID = profileID
	meta.RootFolderPath = root
	meta.Path = folder
	meta.FolderName = filepath.Base(folder)
	meta.Monitored = true
	meta.MinimumAvailability = "released"
	return opts.Store.CreateMovie(meta)
}

func resolveSeries(ctx context.Context, opts ImportOptions, it ImportItem, profileID int64, src string) (*domain.Series, error) {
	if it.MatchID > 0 {
		return opts.Store.GetSeriesByID(it.MatchID)
	}
	tvdbID := it.TvdbID
	if tvdbID > 0 {
		if existing, err := opts.Store.GetSeriesByTvdbID(tvdbID); err == nil && existing != nil {
			return existing, nil
		}
	}
	if tvdbID <= 0 || opts.Meta == nil {
		return nil, fmt.Errorf("series match required")
	}
	meta, _, _, err := opts.Meta.GetSeriesByTVDB(ctx, tvdbID, opts.Language)
	if err != nil {
		return nil, err
	}
	// Prefer series folder (grandparent of episode file when under Season XX/).
	folder := filepath.Dir(src)
	base := filepath.Base(folder)
	if strings.HasPrefix(strings.ToLower(base), "season") {
		folder = filepath.Dir(folder)
	}
	root := strings.TrimSpace(opts.RootFolderPath)
	if root == "" {
		root = folder
	}
	meta.QualityProfileID = profileID
	meta.RootFolderPath = root
	meta.Path = folder
	meta.Monitored = true
	meta.SeasonFolder = true
	created, err := opts.Store.CreateSeries(meta)
	if err != nil {
		return nil, err
	}
	if opts.SyncSeriesEpisodes != nil {
		_ = opts.SyncSeriesEpisodes(ctx, created)
	}
	return created, nil
}

func resolveArtist(ctx context.Context, opts ImportOptions, it ImportItem, profileID int64, src string) (*domain.Artist, error) {
	if it.MatchID > 0 {
		return opts.Store.GetArtistByID(it.MatchID)
	}
	foreignID := strings.TrimSpace(it.ForeignID)
	if foreignID != "" {
		if existing, err := opts.Store.GetArtistByForeignID(foreignID); err == nil && existing != nil {
			return existing, nil
		}
	}
	if foreignID == "" || opts.Meta == nil {
		return nil, fmt.Errorf("artist match required")
	}
	meta, _, err := opts.Meta.GetArtistByMBID(ctx, foreignID)
	if err != nil {
		return nil, err
	}
	folder := filepath.Dir(src)
	// Prefer artist folder when file is nested Artist/Album/track.
	rel, err := filepath.Rel(opts.RootFolderPath, src)
	if err == nil {
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) >= 2 && opts.RootFolderPath != "" {
			folder = filepath.Join(opts.RootFolderPath, parts[0])
		}
	}
	root := strings.TrimSpace(opts.RootFolderPath)
	if root == "" {
		root = folder
	}
	meta.QualityProfileID = profileID
	meta.RootFolderPath = root
	meta.Path = folder
	meta.Monitored = true
	created, err := opts.Store.CreateArtist(meta)
	if err != nil {
		return nil, err
	}
	if opts.SyncArtistAlbums != nil {
		_ = opts.SyncArtistAlbums(ctx, created)
	}
	return created, nil
}

func resolveBook(ctx context.Context, opts ImportOptions, it ImportItem, profileID int64, src, mediaType string) (*domain.Book, error) {
	if it.MatchID > 0 {
		if bk, err := opts.Store.GetBookByID(it.MatchID); err == nil && bk != nil {
			_ = ensureBookEdition(opts, bk, src, it.Format, mediaType, profileID)
			return bk, nil
		}
	}
	foreignID := strings.TrimSpace(it.ForeignID)
	if foreignID != "" {
		if existing, err := opts.Store.GetBookByForeignID(foreignID); err == nil && existing != nil {
			_ = ensureBookEdition(opts, existing, src, it.Format, mediaType, profileID)
			return existing, nil
		}
	}
	if foreignID == "" || opts.Meta == nil {
		return nil, fmt.Errorf("book match required")
	}
	book, author, _, err := opts.Meta.GetBookByWorkKey(ctx, foreignID)
	if err != nil {
		return nil, err
	}
	format := strings.TrimSpace(it.Format)
	if format == "" {
		format = detectBookFormat(src, mediaType)
	}
	folder := filepath.Dir(src)
	root := strings.TrimSpace(opts.RootFolderPath)
	if root == "" {
		root = folder
	}

	var authorID int64
	if author.ForeignAuthorID != "" {
		if existing, err := opts.Store.GetAuthorByForeignID(author.ForeignAuthorID); err == nil && existing != nil {
			authorID = existing.ID
		}
	}
	if authorID == 0 {
		if author.Title == "" {
			author.Title = "Unknown Author"
		}
		author.RootFolderPath = root
		author.Path = folder
		author.QualityProfileID = profileID
		author.Monitored = true
		createdAuthor, err := opts.Store.CreateAuthor(author)
		if err != nil {
			return nil, err
		}
		authorID = createdAuthor.ID
	}

	book.AuthorID = authorID
	book.Monitored = true
	createdBook, err := opts.Store.CreateBook(book)
	if err != nil {
		return nil, err
	}
	_ = ensureBookEdition(opts, createdBook, src, format, mediaType, profileID)
	return createdBook, nil
}

func ensureBookEdition(opts ImportOptions, book *domain.Book, src, format, mediaType string, profileID int64) error {
	if book == nil {
		return fmt.Errorf("book required")
	}
	format = strings.TrimSpace(format)
	if format == "" {
		format = detectBookFormat(src, mediaType)
	}
	folder := filepath.Dir(src)
	root := strings.TrimSpace(opts.RootFolderPath)
	if root == "" {
		root = folder
	}
	editions, err := opts.Store.ListEditions(book.ID, format)
	if err == nil {
		for i := range editions {
			ed := editions[i]
			ed.HasFile = true
			ed.Path = folder
			ed.RootFolderPath = root
			if ed.QualityProfileID == 0 {
				ed.QualityProfileID = profileID
			}
			_, _ = opts.Store.UpdateEdition(ed)
			return nil
		}
	}
	_, err = opts.Store.CreateEdition(domain.Edition{
		BookID:           book.ID,
		AuthorID:         book.AuthorID,
		ForeignEditionID: book.ForeignBookID + "-" + format,
		Title:            book.Title,
		Format:           format,
		Monitored:        true,
		HasFile:          true,
		Path:             folder,
		QualityProfileID: profileID,
		RootFolderPath:   root,
		Images:           book.Images,
		ReleaseDate:      book.ReleaseDate,
		PageCount:        book.PageCount,
	})
	return err
}
