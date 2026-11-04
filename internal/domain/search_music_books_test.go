package domain

import "testing"

func TestBuildMusicSearchQueries(t *testing.T) {
	got := BuildMusicSearchQueries("Radiohead", "OK Computer", "")
	if len(got) < 2 {
		t.Fatalf("expected artist+album variants, got %#v", got)
	}
	if got[0] != "Radiohead OK Computer" {
		t.Fatalf("first query = %q", got[0])
	}
}

func TestBuildBookSearchQueries(t *testing.T) {
	got := BuildBookSearchQueries("Tolkien", "The Hobbit")
	if len(got) < 2 {
		t.Fatalf("expected author+title variants, got %#v", got)
	}
	if got[0] != "Tolkien The Hobbit" {
		t.Fatalf("first query = %q", got[0])
	}
}
