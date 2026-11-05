package libraryimport

import "testing"

func TestParseMediaName(t *testing.T) {
	p := parseMediaName("Dune Part Two (2024) {tmdb-693134}")
	if p.Title != "Dune Part Two" || p.Year != 2024 || p.TmdbID != 693134 {
		t.Fatalf("got %+v", p)
	}

	p = parseMediaName("The.Matrix.1999.1080p.BluRay.x264")
	if p.Title != "The Matrix" || p.Year != 1999 {
		t.Fatalf("release-style got %+v", p)
	}

	p = parseMediaName("Arrival (2016) {imdb-tt2543164}")
	if p.Title != "Arrival" || p.Year != 2016 || p.ImdbID != "tt2543164" {
		t.Fatalf("imdb tag got %+v", p)
	}
}

func TestGuessNameForFile(t *testing.T) {
	p := guessNameForFile("/data/Movies", "/data/Movies/Dune Part Two (2024)/Dune.mkv")
	if p.Title != "Dune Part Two" || p.Year != 2024 {
		t.Fatalf("folder guess %+v", p)
	}
	p = guessNameForFile("/data/Movies", "/data/Movies/Flat.Movie.2020.mkv")
	if p.Title != "Flat Movie" || p.Year != 2020 {
		t.Fatalf("flat guess %+v", p)
	}
}
