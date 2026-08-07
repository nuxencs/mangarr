package source

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testMangaDexMangaID = "801513ba-a712-498c-8f57-cae55b38cc92"
	testMangaDexGroupID = "277df5c9-a486-40f6-8dfa-c086c6b60935"
)

func TestMangaDexPaginatesBeforeRejectingFilteredResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/manga/" + testMangaDexMangaID:
			name = "mangadex-manga.json"
		case "/manga/" + testMangaDexMangaID + "/feed":
			if r.URL.Query().Get("offset") == "0" {
				name = "mangadex-feed-page-1.json"
			} else {
				name = "mangadex-feed-page-2.json"
			}
		case "/at-home/server/chapter-selected":
			name = "mangadex-pages.json"
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, name))
	}))
	defer server.Close()

	source := NewMangadex(testMangaDexMangaID, testMangaDexGroupID, "en").(*mangadex)
	source.BaseURL = server.URL
	source.Client = server.Client()

	manga, err := source.GetManga(t.Context())
	if err != nil {
		t.Fatalf("get manga: %v", err)
	}
	if manga.Title != "Fixture English" {
		t.Fatalf("manga title = %q", manga.Title)
	}
	if err := source.GetChapters(t.Context(), manga); err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(manga.Chapters) != 1 {
		t.Fatalf("chapter count = %d, want 1", len(manga.Chapters))
	}

	for _, chapter := range manga.Chapters {
		if chapter.ID != "chapter-selected" {
			t.Fatalf("chapter ID = %q", chapter.ID)
		}
		if err := source.GetImageURLs(t.Context(), &chapter); err != nil {
			t.Fatalf("get image URLs: %v", err)
		}
		if len(chapter.ImageInfo) != 2 {
			t.Fatalf("image count = %d, want 2", len(chapter.ImageInfo))
		}
	}
}

func TestMangaDexValidatesOptionalGroupID(t *testing.T) {
	source := NewMangadex(testMangaDexMangaID, "invalid", "en")
	if err := source.ValidateInput(); err == nil {
		t.Fatal("expected invalid group ID to fail validation")
	}
}
