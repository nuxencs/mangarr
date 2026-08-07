package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTCBScansFixtureFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/projects":
			name = "tcb-projects.html"
		case "/manga/one-piece":
			name = "tcb-chapters.html"
		case "/chapters/1":
			name = "tcb-images.html"
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixture(t, name))
	}))
	defer server.Close()

	source := NewTCBScans("One Piece").(*tcbscans)
	source.BaseURL = server.URL

	manga, err := source.GetManga(t.Context())
	if err != nil {
		t.Fatalf("get manga: %v", err)
	}
	if err := source.GetChapters(t.Context(), manga); err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(manga.Chapters) != 1 {
		t.Fatalf("chapter count = %d, want 1", len(manga.Chapters))
	}

	for _, chapter := range manga.Chapters {
		if err := source.GetImageURLs(t.Context(), &chapter); err != nil {
			t.Fatalf("get image URLs: %v", err)
		}
		if len(chapter.ImageInfo) != 2 {
			t.Fatalf("image count = %d, want 2", len(chapter.ImageInfo))
		}
	}
}

func TestTCBScansHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	source := NewTCBScans("One Piece").(*tcbscans)
	source.BaseURL = server.URL
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if _, err := source.GetManga(ctx); err == nil {
		t.Fatal("expected canceled scrape to fail")
	}
}
