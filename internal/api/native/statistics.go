package native

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Server) computeStatistics(mediaType string) (domain.MediaStatistics, error) {
	now := time.Now().UTC()
	stats := domain.MediaStatistics{
		MediaType:              mediaType,
		MissingItems:           []domain.MissingStatItem{},
		QualityBreakdown:       []domain.QualityStat{},
		StatusBreakdown:        []domain.NamedCount{},
		SeriesTypeBreakdown:    []domain.NamedCount{},
		EpisodeStatusBreakdown: []domain.NamedCount{},
		QualityProfiles:        []domain.NamedCount{},
		Genres:                 []domain.NamedCount{},
		FileTypes:              []domain.NamedCount{},
	}

	profiles, _ := s.store.ListQualityProfiles()
	profileNames := map[int64]string{}
	for _, qp := range profiles {
		profileNames[qp.ID] = qp.Name
	}

	switch mediaType {
	case "tv", "series":
		stats.MediaType = "tv"
		series, err := s.store.ListSeriesBasic()
		if err != nil {
			return stats, err
		}
		stats.ItemCount = len(series)

		statusCounts := map[string]int{}
		typeCounts := map[string]int{}
		profileCounts := map[string]int{}
		genreCounts := map[string]int{}
		epStatus := map[string]int{}
		qualityCounts := map[string]int{}
		qualitySizes := map[string]int64{}
		seriesByID := map[int64]domain.Series{}

		for _, sr := range series {
			seriesByID[sr.ID] = sr
			if sr.Monitored {
				stats.MonitoredCount++
			}
			statusCounts[normalizeLibraryStatus(sr.Status, sr.Ended)]++
			typeCounts[normalizeSeriesType(sr.SeriesType)]++
			profileCounts[profileLabel(profileNames, sr.QualityProfileID)]++
			bumpGenres(genreCounts, sr.Genres)
		}

		eps, err := s.store.ListEpisodes(0)
		if err != nil {
			return stats, err
		}
		for _, ep := range eps {
			switch {
			case ep.HasFile:
				stats.WithFileCount++
				epStatus["downloaded"]++
			case !ep.Monitored:
				epStatus["unmonitored"]++
			default:
				air := ep.AirDateUtc
				if air.IsZero() {
					air, _ = time.Parse("2006-01-02", ep.AirDate)
				}
				if !air.IsZero() && air.After(now) {
					stats.UnreleasedCount++
					epStatus["unaired"]++
				} else {
					stats.MissingCount++
					epStatus["missing"]++
					if len(stats.MissingItems) < 100 {
						title := fmt.Sprintf("Series %d", ep.SeriesID)
						if sr, ok := seriesByID[ep.SeriesID]; ok {
							title = sr.Title
						}
						stats.MissingItems = append(stats.MissingItems, domain.MissingStatItem{
							ID: ep.SeriesID, Title: title,
							Detail: fmt.Sprintf("S%02dE%02d %s", ep.SeasonNumber, ep.EpisodeNumber, ep.Title),
						})
					}
				}
			}
		}

		files, err := s.store.ListEpisodeFiles(0)
		if err != nil {
			return stats, err
		}
		var size int64
		for _, f := range files {
			size += f.Size
			q := inferReleaseQuality(f.SceneName, f.Path)
			qualityCounts[q]++
			qualitySizes[q] += f.Size
		}
		stats.SizeOnDisk = size
		stats.AverageFileSize = averageSize(size, len(files))
		stats.StatusBreakdown = orderedNamedCounts(statusCounts, []string{"continuing", "ended", "upcoming", "deleted"})
		stats.SeriesTypeBreakdown = orderedNamedCounts(typeCounts, []string{"standard", "daily", "anime"})
		stats.EpisodeStatusBreakdown = orderedNamedCounts(epStatus, []string{"downloaded", "missing", "unaired", "unmonitored"})
		stats.QualityProfiles = sortedNamedCounts(profileCounts)
		stats.QualityBreakdown = sortedQualityStats(qualityCounts, qualitySizes)
		stats.Genres = topNamedCounts(genreCounts, 15)

	case "movies", "movie":
		stats.MediaType = "movies"
		movies, err := s.store.ListMovies()
		if err != nil {
			return stats, err
		}
		stats.ItemCount = len(movies)

		statusCounts := map[string]int{}
		profileCounts := map[string]int{}
		genreCounts := map[string]int{}
		qualityCounts := map[string]int{}
		qualitySizes := map[string]int64{}

		for _, mv := range movies {
			if mv.Monitored {
				stats.MonitoredCount++
			}
			statusCounts[normalizeMovieStatus(mv.Status)]++
			profileCounts[profileLabel(profileNames, mv.QualityProfileID)]++
			bumpGenres(genreCounts, mv.Genres)

			if mv.HasFile {
				stats.WithFileCount++
				stats.SizeOnDisk += mv.SizeOnDisk
			} else if mv.Monitored {
				stats.MissingCount++
				if len(stats.MissingItems) < 100 {
					stats.MissingItems = append(stats.MissingItems, domain.MissingStatItem{ID: mv.ID, Title: mv.Title})
				}
			}
		}

		files, _ := s.store.ListMovieFiles(0)
		var fileSizeTotal int64
		for _, f := range files {
			q := inferReleaseQuality(f.SceneName, f.Path)
			qualityCounts[q]++
			qualitySizes[q] += f.Size
			fileSizeTotal += f.Size
		}
		if stats.SizeOnDisk == 0 && fileSizeTotal > 0 {
			stats.SizeOnDisk = fileSizeTotal
		}
		avgCount := len(files)
		if avgCount == 0 {
			avgCount = stats.WithFileCount
		}
		stats.AverageFileSize = averageSize(stats.SizeOnDisk, avgCount)

		stats.StatusBreakdown = orderedNamedCounts(statusCounts, []string{"continuing", "ended", "upcoming", "deleted"})
		stats.QualityProfiles = sortedNamedCounts(profileCounts)
		stats.QualityBreakdown = sortedQualityStats(qualityCounts, qualitySizes)
		stats.Genres = topNamedCounts(genreCounts, 15)

	case "music":
		artists, err := s.store.ListArtists()
		if err != nil {
			return stats, err
		}
		stats.ItemCount = len(artists)

		statusCounts := map[string]int{}
		genreCounts := map[string]int{}

		for _, a := range artists {
			if a.Monitored {
				stats.MonitoredCount++
			}
			statusCounts[normalizeLibraryStatus(a.Status, false)]++
			bumpGenres(genreCounts, a.Genres)

			albums, _ := s.store.ListAlbums(a.ID)
			for _, al := range albums {
				bumpGenres(genreCounts, al.Genres)
				if al.HasFile {
					stats.WithFileCount++
				} else if al.Monitored {
					stats.MissingCount++
					if len(stats.MissingItems) < 100 {
						stats.MissingItems = append(stats.MissingItems, domain.MissingStatItem{
							ID: a.ID, Title: a.Title, Detail: al.Title,
						})
					}
				}
			}
		}
		stats.StatusBreakdown = orderedNamedCounts(statusCounts, []string{"continuing", "ended", "upcoming", "deleted"})
		stats.Genres = topNamedCounts(genreCounts, 15)

	case "books":
		books, err := s.store.ListBooks(0)
		if err != nil {
			return stats, err
		}
		stats.ItemCount = len(books)

		authors, _ := s.store.ListAuthors()
		statusCounts := map[string]int{}
		genreCounts := map[string]int{}
		fileTypes := map[string]int{}

		for _, a := range authors {
			if a.Monitored {
				stats.MonitoredCount++
			}
			statusCounts[normalizeLibraryStatus(a.Status, false)]++
			bumpGenres(genreCounts, a.Genres)
		}
		for _, b := range books {
			bumpGenres(genreCounts, b.Genres)
			eds, _ := s.store.ListEditions(b.ID, "")
			has := false
			for _, e := range eds {
				if e.HasFile {
					has = true
					ft := inferBookFileType(e.Path, e.Format)
					fileTypes[ft]++
				}
			}
			if has {
				stats.WithFileCount++
			} else if b.Monitored {
				stats.MissingCount++
				if len(stats.MissingItems) < 100 {
					stats.MissingItems = append(stats.MissingItems, domain.MissingStatItem{ID: b.ID, Title: b.Title})
				}
			}
		}
		stats.StatusBreakdown = orderedNamedCounts(statusCounts, []string{"continuing", "ended", "upcoming", "deleted"})
		stats.Genres = topNamedCounts(genreCounts, 15)
		stats.FileTypes = sortedNamedCounts(fileTypes)

	default:
		return stats, fmt.Errorf("unsupported mediaType")
	}
	return stats, nil
}

func averageSize(total int64, count int) int64 {
	if count <= 0 || total <= 0 {
		return 0
	}
	return total / int64(count)
}

func profileLabel(names map[int64]string, id int64) string {
	if name, ok := names[id]; ok && name != "" {
		return name
	}
	if id == 0 {
		return "None"
	}
	return fmt.Sprintf("Profile %d", id)
}

func normalizeSeriesType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "daily":
		return "daily"
	case "anime":
		return "anime"
	default:
		return "standard"
	}
}

func normalizeLibraryStatus(status string, ended bool) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "continuing", "ended", "upcoming", "deleted":
		return s
	case "announced":
		return "upcoming"
	case "released":
		return "ended"
	}
	if ended {
		return "ended"
	}
	if s == "" {
		return "continuing"
	}
	return "continuing"
}

func normalizeMovieStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "continuing", "incinemas":
		return "continuing"
	case "ended", "released":
		return "ended"
	case "upcoming", "announced":
		return "upcoming"
	case "deleted":
		return "deleted"
	case "":
		return "ended"
	default:
		return normalizeLibraryStatus(status, false)
	}
}

func bumpGenres(dst map[string]int, genres []string) {
	for _, g := range genres {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		dst[g]++
	}
}

func orderedNamedCounts(counts map[string]int, order []string) []domain.NamedCount {
	out := make([]domain.NamedCount, 0, len(order))
	seen := map[string]bool{}
	for _, name := range order {
		seen[name] = true
		out = append(out, domain.NamedCount{Name: name, Count: counts[name]})
	}
	var extra []string
	for name := range counts {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		out = append(out, domain.NamedCount{Name: name, Count: counts[name]})
	}
	return out
}

func sortedNamedCounts(counts map[string]int) []domain.NamedCount {
	out := make([]domain.NamedCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, domain.NamedCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func topNamedCounts(counts map[string]int, limit int) []domain.NamedCount {
	out := sortedNamedCounts(counts)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortedQualityStats(counts map[string]int, sizes map[string]int64) []domain.QualityStat {
	out := make([]domain.QualityStat, 0, len(counts))
	for name, count := range counts {
		out = append(out, domain.QualityStat{Name: name, Count: count, Size: sizes[name]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func inferReleaseQuality(sceneName, path string) string {
	hay := strings.ToLower(sceneName + " " + filepath.Base(path))
	switch {
	case strings.Contains(hay, "2160p") && strings.Contains(hay, "remux"):
		return "Bluray-2160p Remux"
	case strings.Contains(hay, "1080p") && strings.Contains(hay, "remux"):
		return "Bluray-1080p Remux"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "bluray") || strings.Contains(hay, "blu-ray") || strings.Contains(hay, "bdrip")):
		return "Bluray-720p"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "webrip") || strings.Contains(hay, "web-rip")):
		return "WEBRip-720p"
	case strings.Contains(hay, "2160p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-2160p"
	case strings.Contains(hay, "1080p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-1080p"
	case strings.Contains(hay, "720p") && (strings.Contains(hay, "web-dl") || strings.Contains(hay, "webdl") || strings.Contains(hay, "web.dl")):
		return "WEBDL-720p"
	case strings.Contains(hay, "1080p") && strings.Contains(hay, "hdtv"):
		return "HDTV-1080p"
	case strings.Contains(hay, "720p") && strings.Contains(hay, "hdtv"):
		return "HDTV-720p"
	case strings.Contains(hay, "2160p") || strings.Contains(hay, "4k") || strings.Contains(hay, "uhd"):
		return "2160p"
	case strings.Contains(hay, "1080p"):
		return "1080p"
	case strings.Contains(hay, "720p"):
		return "720p"
	case strings.Contains(hay, "480p"):
		return "480p"
	default:
		return "Unknown"
	}
}

func inferBookFileType(path, format string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "epub", "mobi", "azw", "azw3", "pdf", "cbz", "cbr", "txt", "mp3", "m4b", "m4a", "flac", "ogg", "opus":
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook":
		return "audiobook"
	case "ebook":
		return "ebook"
	}
	if ext != "" {
		return ext
	}
	return "unknown"
}
