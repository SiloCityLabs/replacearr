package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchSeriesFreeText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("term") != "severance" {
			t.Fatalf("term = %q", r.URL.Query().Get("term"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"tvdbId": 371980,
				"title": "Severance",
				"overview": "Mark leads a team.",
				"slug": "severance",
				"firstAired": "2022-02-18",
				"imdbId": "tt11280740",
				"status": "Continuing",
				"runtime": 49,
				"network": "Apple TV",
				"genres": ["Drama"],
				"images": [{"coverType": "Poster", "url": "https://example.com/poster.jpg"}]
			}
		]`))
	}))
	defer ts.Close()

	svc := New(WithSkyHookBase(ts.URL), WithHTTPClient(ts.Client()))
	got, err := svc.SearchSeries(context.Background(), "Severance", "english")
	if err != nil {
		t.Fatalf("SearchSeries: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Severance" || got[0].TvdbID != 371980 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got[0].Year != 2022 {
		t.Fatalf("year = %d, want 2022", got[0].Year)
	}
	if len(got[0].Images) != 1 || got[0].Images[0].CoverType != "poster" {
		t.Fatalf("images = %+v", got[0].Images)
	}
}

func TestSearchSeriesByTVDB(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shows/eng/371980/official" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tvdbId": 371980,
			"title": "Severance",
			"firstAired": "2022-02-18",
			"status": "Continuing",
			"runtime": 50,
			"network": "Apple TV+",
			"imdbId": "tt11280740",
			"seasons": [{"seasonNumber": 1}, {"seasonNumber": 2}],
			"episodes": [
				{"tvdbId": 1, "seasonNumber": 1, "episodeNumber": 1, "title": "Good News About Hell", "airDate": "2022-02-18", "airDateUtc": "2022-02-18T12:00:00Z"}
			]
		}`))
	}))
	defer ts.Close()

	svc := New(WithSkyHookBase(ts.URL), WithHTTPClient(ts.Client()))
	got, err := svc.SearchSeries(context.Background(), "tvdb:371980", "en")
	if err != nil {
		t.Fatalf("SearchSeries: %v", err)
	}
	if len(got) != 1 || got[0].TvdbID != 371980 || len(got[0].Seasons) != 2 {
		t.Fatalf("unexpected: %+v", got)
	}
	series, eps, err := svc.GetSeriesByTVDB(context.Background(), 371980, "en")
	if err != nil || series.Title != "Severance" || len(eps) != 1 || eps[0].Title != "Good News About Hell" {
		t.Fatalf("GetSeriesByTVDB: %v series=%+v eps=%+v", err, series, eps)
	}
}

func TestSearchMoviesFreeTextAndPrefixes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/search":
			_, _ = w.Write([]byte(`[
				{
					"TmdbId": 693134,
					"ImdbId": "tt15239678",
					"Title": "Dune: Part Two",
					"Overview": "Paul.",
					"Year": 2024,
					"Runtime": 166,
					"Studio": "Legendary",
					"Status": "released",
					"Images": [{"CoverType": "Poster", "Url": "https://example.com/dune.jpg"}]
				}
			]`))
		case r.URL.Path == "/movie/693134":
			_, _ = w.Write([]byte(`{
				"TmdbId": 693134,
				"ImdbId": "tt15239678",
				"Title": "Dune: Part Two",
				"Year": 2024,
				"Status": "released"
			}`))
		case r.URL.Path == "/movie/imdb/tt15239678":
			_, _ = w.Write([]byte(`[
				{"TmdbId": 693134, "ImdbId": "tt15239678", "Title": "Dune: Part Two", "Year": 2024}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	svc := New(WithRadarrBase(ts.URL), WithHTTPClient(ts.Client()))

	free, err := svc.SearchMovies(context.Background(), "Dune Part Two")
	if err != nil {
		t.Fatalf("free text: %v", err)
	}
	if len(free) != 1 || free[0].TmdbID != 693134 || free[0].Images[0].CoverType != "poster" {
		t.Fatalf("free text result: %+v", free)
	}

	byTMDB, err := svc.SearchMovies(context.Background(), "tmdb:693134")
	if err != nil || len(byTMDB) != 1 || byTMDB[0].Title != "Dune: Part Two" {
		t.Fatalf("tmdb prefix: %v %+v", err, byTMDB)
	}

	byIMDB, err := svc.SearchMovies(context.Background(), "imdb:tt15239678")
	if err != nil || len(byIMDB) != 1 || byIMDB[0].TmdbID != 693134 {
		t.Fatalf("imdb prefix: %v %+v", err, byIMDB)
	}

	byURL, err := svc.SearchMovies(context.Background(), "https://www.imdb.com/title/tt15239678/")
	if err != nil || len(byURL) != 1 {
		t.Fatalf("imdb url: %v %+v", err, byURL)
	}
}

func TestEmptyTerm(t *testing.T) {
	svc := New()
	series, err := svc.SearchSeries(context.Background(), "  ", "en")
	if err != nil || len(series) != 0 {
		t.Fatalf("empty series: %v %+v", err, series)
	}
	movies, err := svc.SearchMovies(context.Background(), "")
	if err != nil || len(movies) != 0 {
		t.Fatalf("empty movies: %v %+v", err, movies)
	}
}

func TestLanguageCode(t *testing.T) {
	if LanguageCode("english") != "eng" {
		t.Fatalf("english")
	}
	if LanguageCode("de-DE") != "deu" {
		t.Fatalf("de-DE")
	}
}
