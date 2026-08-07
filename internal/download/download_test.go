package download

import (
	"bytes"
	"context"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"mangarr/internal/domain"
	"mangarr/internal/sharedhttp"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestDownloadImageStreamsBodyToDisk(t *testing.T) {
	t.Parallel()

	payload := []byte("stream-me-directly")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(context.Background(), zerolog.Nop(), domain.ImageInfo{ImageURL: server.URL}, outBase, 1, 1)
	require.NoError(t, err)

	got, err := os.ReadFile(outBase + ".png")
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestDownloadImageDecryptsWhileStreaming(t *testing.T) {
	t.Parallel()

	plain := []byte("decrypt-me-while-streaming")
	key := []byte{0x12, 0x34, 0x56}
	encrypted := xorBytes(plain, key)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(encrypted)
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(context.Background(), zerolog.Nop(), domain.ImageInfo{
		ImageURL:      server.URL,
		EncryptionKey: hex.EncodeToString(key),
	}, outBase, 1, 1)
	require.NoError(t, err)

	got, err := os.ReadFile(outBase + ".jpg")
	require.NoError(t, err)
	require.Equal(t, plain, got)
}

func TestDownloadImageDetectsTypeFromMagicBytes(t *testing.T) {
	t.Parallel()

	var pngBytes bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&pngBytes, img))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(pngBytes.Bytes())
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(context.Background(), zerolog.Nop(), domain.ImageInfo{ImageURL: server.URL}, outBase, 1, 1)
	require.NoError(t, err)

	_, statErr := os.Stat(outBase + ".png")
	require.NoError(t, statErr)
}

func TestDownloadImageAppliesHeadersAndProcessor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Image-Test") != "present" {
			t.Errorf("missing image request header")
		}
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(t.Context(), zerolog.Nop(), domain.ImageInfo{
		ImageURL:       server.URL,
		RequestHeaders: map[string]string{"X-Image-Test": "present"},
		Processor:      testImageProcessor{},
	}, outBase, 1, 1)
	require.NoError(t, err)

	result, err := os.ReadFile(outBase + ".processed")
	require.NoError(t, err)
	require.Equal(t, []byte("processed:payload"), result)
}

func TestDownloadImageRetriesTransientServerError(t *testing.T) {
	t.Parallel()

	payload := []byte("retry-success")
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(context.Background(), zerolog.Nop(), domain.ImageInfo{ImageURL: server.URL}, outBase, 1, 1)
	require.NoError(t, err)

	got, err := os.ReadFile(outBase + ".png")
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.Equal(t, int32(2), attempts.Load())
}

func TestDownloadImageStopsAfterThreeAttemptsOnPermanentFailure(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	outBase := filepath.Join(t.TempDir(), "img")
	err := downloadImage(context.Background(), zerolog.Nop(), domain.ImageInfo{ImageURL: server.URL}, outBase, 1, 1)
	require.Error(t, err)
	require.Equal(t, int32(sharedhttp.RetryAttempts), attempts.Load())
}

func xorBytes(data, key []byte) []byte {
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ key[i%len(key)]
	}

	return out
}

type testImageProcessor struct{}

func (testImageProcessor) Extension() string {
	return ".processed"
}

func (testImageProcessor) Process(_ http.Header, reader io.Reader, writer io.Writer) error {
	if _, err := io.WriteString(writer, "processed:"); err != nil {
		return err
	}
	_, err := io.Copy(writer, reader)

	return err
}
