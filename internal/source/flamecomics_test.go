package source

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlameComicsFixtureFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/series/fixture":
			name = "flame-series.html"
		case "/series/7/chapter-token":
			name = "flame-chapter.html"
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixture(t, name))
	}))
	defer server.Close()

	source := NewFlamecomics(server.URL + "/series/fixture").(*flamecomics)
	source.BaseURL = server.URL
	source.CDNBaseURL = "https://cdn.example"

	manga, err := source.Discover(t.Context())
	if err != nil {
		t.Fatalf("get manga: %v", err)
	}
	if manga.Title != "Fixture Series" {
		t.Fatalf("manga title = %q", manga.Title)
	}
	for _, chapter := range manga.Chapters {
		pages, err := source.Pages(t.Context(), chapter)
		if err != nil {
			t.Fatalf("get image URLs: %v", err)
		}
		if got := pages[0].ImageURL; got != "https://cdn.example/uploads/images/series/7/chapter-token/001.webp" {
			t.Fatalf("first image URL = %q", got)
		}
	}
}

func TestFlameComicsRejectsPrefixHostSpoofing(t *testing.T) {
	source := NewFlamecomics("https://flamecomics.xyz.example/series/fixture")
	if err := source.ValidateInput(); err == nil {
		t.Fatal("expected spoofed host to fail validation")
	}
}
