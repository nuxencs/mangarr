package source

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestComixValidateInput(t *testing.T) {
	t.Parallel()

	src := NewComix("https://comix.to/title/pvry-one-piece", "")
	require.NoError(t, src.ValidateInput())

	src = NewComix("https://example.com/title/pvry-one-piece", "")
	require.EqualError(t, src.ValidateInput(), "URL must use https://comix.to")

	src = NewComix("https://comix.to/title/pvry-one-piece", "not-numeric")
	require.ErrorContains(t, src.ValidateInput(), "Comix group ID must be numeric")
}

func TestComixCodecTokenMatchesFrontendBuild(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	token, err := codec.token("https://comix.to/api/v1/manga/pvry/chapters", comixParams{
		"page":  1,
		"limit": 20,
		"order": map[string]any{"number": "desc"},
	})
	require.NoError(t, err)
	require.Equal(t, "IZ-P1pUtAjt3Oig5K1xNJ4A3XDWhN5iWaB2v65-B44CevAKBEF0OAOg-407yPS4VFzRxRsBwdg", token)
}

func TestCanonicalComixRequestSortsAndFlattensParameters(t *testing.T) {
	t.Parallel()

	input, err := canonicalComixRequest("/api/v1/manga/pvry?ignored=true", comixParams{
		"rating": []string{"safe", "suggestive"},
		"page":   2,
		"order":  map[string]any{"number": "asc"},
		"nil":    nil,
		"_":      "old-token",
	})
	require.NoError(t, err)
	require.Equal(t, "/manga/pvry?order[number]=asc&page=2&rating[0]=safe&rating[1]=suggestive", input)
}

func TestComixCodecDecodesEncryptedResponse(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	payload := []byte(`{"status":"ok","result":{"id":366,"hid":"pvry","title":"One Piece"}}`)
	encoded := payload
	for _, stage := range codec.stages {
		encoded = stage.encode(encoded)
	}
	envelope, err := json.Marshal(map[string]string{"e": base64.RawURLEncoding.EncodeToString(encoded)})
	require.NoError(t, err)

	var result struct {
		ID    int    `json:"id"`
		HID   string `json:"hid"`
		Title string `json:"title"`
	}
	require.NoError(t, codec.decodeResponse(bytes.NewReader(envelope), true, &result))
	require.Equal(t, 366, result.ID)
	require.Equal(t, "pvry", result.HID)
	require.Equal(t, "One Piece", result.Title)
}

func TestComixCodecDecodesPlainResponse(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	var result struct {
		HID string `json:"hid"`
	}
	err = codec.decodeResponse(bytes.NewBufferString(`{"status":"ok","result":{"hid":"pvry"}}`), false, &result)
	require.NoError(t, err)
	require.Equal(t, "pvry", result.HID)
}

func TestComixCodecRejectsUnsupportedPath(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	_, err = codec.token("https://comix.to/api/v1/user", nil)
	require.EqualError(t, err, `unsupported Comix token path "/user"`)

	_, err = codec.token("https://comix.to/api/v1/chapters/", nil)
	require.EqualError(t, err, `unsupported Comix token path "/chapters/"`)
}

func TestComixCodecRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	err = codec.decodeResponse(bytes.NewBufferString(`{"e":"not valid!"}`), true, &struct{}{})
	require.ErrorContains(t, err, "decoding Comix encrypted payload")
}

func TestComixGetMangaUsesCurrentEncryptedAPI(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/manga/pvry" {
			t.Errorf("path = %q, want /api/v1/manga/pvry", r.URL.Path)
		}
		if r.URL.Query().Get("_") != "IZ-P1pUtAjt3Oig" {
			t.Errorf("unexpected token %q", r.URL.Query().Get("_"))
		}
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Errorf("missing XMLHttpRequest header")
		}
		writeEncryptedComixResponse(t, w, codec, map[string]any{
			"id":    366,
			"hid":   "pvry",
			"title": "One: Piece",
			"type":  "manga",
			"url":   "/title/pvry-one-piece",
		})
	}))
	defer server.Close()

	source := newTestComix(t, server, "https://comix.to/title/pvry-one-piece", "")
	manga, err := source.GetManga(t.Context())
	require.NoError(t, err)
	require.Equal(t, "pvry", manga.ID)
	require.Equal(t, "One Piece", manga.Title)
	require.Equal(t, server.URL+"/title/pvry-one-piece", manga.URL)
	require.False(t, manga.IsManhwa)
	require.NotNil(t, manga.Chapters)
}

func TestComixGetChaptersPaginatesAndFiltersGroup(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			t.Errorf("invalid page: %v", err)
			return
		}
		params := comixParams{
			"group_id": "6594",
			"limit":    100,
			"order":    map[string]any{"number": "desc"},
			"page":     page,
		}
		expectedToken, err := codec.token(r.URL.Path, params)
		if err != nil {
			t.Errorf("generating expected token: %v", err)
			return
		}
		if r.URL.Query().Get("_") != expectedToken {
			t.Errorf("token = %q, want %q", r.URL.Query().Get("_"), expectedToken)
		}
		if r.URL.Query().Get("order[number]") != "desc" {
			t.Errorf("missing nested order query")
		}

		items := []map[string]any{{
			"id": 100 + page, "mangaId": 366, "number": 12 - page,
			"name": "Chapter", "groupId": 6594, "url": fmt.Sprintf("/chapter/%d", page),
		}}
		if page == 1 {
			items = append(items, map[string]any{
				"id": 999, "mangaId": 366, "number": 9,
				"name": "Wrong group", "groupId": 9431, "url": "/chapter/wrong",
			})
		}
		writeEncryptedComixResponse(t, w, codec, map[string]any{
			"items": items,
			"meta":  map[string]any{"page": page, "lastPage": 2},
		})
	}))
	defer server.Close()

	source := newTestComix(t, server, "https://comix.to/title/pvry-one-piece", "6594")
	manga := domain.Manga{ID: "pvry", Chapters: make(map[domain.ChapterNumber]domain.Chapter)}
	require.NoError(t, source.GetChapters(t.Context(), manga))
	require.Len(t, manga.Chapters, 2)
	require.Equal(t, "101", manga.Chapters[domain.MustParseChapterNumber("11")].ID)
	require.Equal(t, server.URL+"/chapter/2", manga.Chapters[domain.MustParseChapterNumber("10")].URL)
}

func TestComixGetImageURLsNormalizesCompactPages(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeEncryptedComixResponse(t, w, codec, map[string]any{
			"id": 101,
			"pages": map[string]any{
				"baseUrl": "https://images.example",
				"items": []map[string]any{
					{"url": "/one.webp", "width": 800, "height": 1200},
					{"url": "https://cdn.example/two.webp", "width": 800, "height": 1200},
				},
			},
		})
	}))
	defer server.Close()

	source := newTestComix(t, server, "https://comix.to/title/pvry-one-piece", "")
	chapter := domain.Chapter{ID: "101"}
	require.NoError(t, source.GetImageURLs(t.Context(), &chapter))
	require.Len(t, chapter.ImageInfo, 2)
	require.Equal(t, "https://images.example/one.webp", chapter.ImageInfo[0].ImageURL)
	require.Equal(t, map[string]string{"Referer": server.URL + "/"}, chapter.ImageInfo[0].RequestHeaders)
	require.Nil(t, chapter.ImageInfo[0].Processor)
	require.Equal(t, "https://cdn.example/two.webp", chapter.ImageInfo[1].ImageURL)
	require.Equal(t, map[string]string{"Referer": server.URL + "/"}, chapter.ImageInfo[1].RequestHeaders)
	require.Nil(t, chapter.ImageInfo[1].Processor)
}

func TestComixGetImageURLsMarksScrambledPage(t *testing.T) {
	t.Parallel()

	codec, err := newComixCodec()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeEncryptedComixResponse(t, w, codec, map[string]any{
			"id": 101,
			"pages": []map[string]any{
				{"url": "https://images.example/one.webp"},
				{"url": "https://images.example/two.webp", "s": 1},
			},
		})
	}))
	defer server.Close()

	source := newTestComix(t, server, "https://comix.to/title/pvry-one-piece", "")
	chapter := domain.Chapter{ID: "101"}
	require.NoError(t, source.GetImageURLs(t.Context(), &chapter))
	require.Len(t, chapter.ImageInfo, 2)
	require.Nil(t, chapter.ImageInfo[0].Processor)
	require.IsType(t, comixImageProcessor{}, chapter.ImageInfo[1].Processor)
}

func TestComixExtractIDRequiresCanonicalTitleURL(t *testing.T) {
	t.Parallel()

	source := NewComix("https://comix.to/title/pvry-one-piece", "").(*comix)
	mangaID, err := source.extractIDFromURL(source.MangaURL)
	require.NoError(t, err)
	require.Equal(t, "pvry", mangaID)

	_, err = source.extractIDFromURL("https://example.com/title/pvry-one-piece")
	require.EqualError(t, err, "URL must use https://comix.to")
}

func newTestComix(t *testing.T, server *httptest.Server, mangaURL, groupID string) *comix {
	t.Helper()

	source := NewComix(mangaURL, groupID).(*comix)
	source.BaseURL = server.URL
	source.Client = server.Client()

	return source
}

func writeEncryptedComixResponse(t *testing.T, w http.ResponseWriter, codec comixCodec, result any) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"status": "ok", "result": result})
	require.NoError(t, err)
	for _, stage := range codec.stages {
		payload = stage.encode(payload)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-enc", "1")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]string{
		"e": base64.RawURLEncoding.EncodeToString(payload),
	}))
}

func Example_comixCodec_token() {
	codec, err := newComixCodec()
	if err != nil {
		panic(err)
	}

	token, err := codec.token("/api/v1/manga/pvry", nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(token)
	// Output:
	// IZ-P1pUtAjt3Oig
}
