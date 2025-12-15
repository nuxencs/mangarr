package download

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

const (
	maxConcurrentImageDownloads = 10
	userAgent                   = "mangarr"
)

type ArchiveWriter func(log zerolog.Logger, tmpDir, outPath string, isManhwa bool) error

func Chapter(ctx context.Context, log zerolog.Logger, outputPath string, chapter domain.Chapter, isManhwa bool, archiveWriter ArchiveWriter) error {
	tmpDir, err := os.MkdirTemp("", "mangarr-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentImageDownloads)

	for i, img := range chapter.ImageInfo {
		g.Go(func() error {
			base := filepath.Join(tmpDir, fmt.Sprintf("%03d", i+1))
			return downloadImage(ctx, log, img.ImageURL, img.EncryptionKey, base)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := archiveWriter(log, tmpDir, outputPath, isManhwa); err != nil {
		return fmt.Errorf("creating archive: %w", err)
	}

	return nil
}

// downloadImage fetches url, applies XOR–decrypt if xorKeyHex isn't empty, and stores the
// file using the right extension that is inferred from the response headers or magic bytes.
func downloadImage(ctx context.Context, log zerolog.Logger, url, xorKeyHex, filenameNoExt string) error {
	var (
		body        []byte
		contentType string
		filename    string
	)

	if err := fetchWithRetry(ctx, url, func(resp *http.Response) error {
		var fetchErr error

		contentType = resp.Header.Get("Content-Type")

		// If we only need to save the stream, we could pipe directly, but for the
		// XOR-decrypt branch we need everything in memory anyway, so always read
		// into a buffer for simplicity.
		body, fetchErr = io.ReadAll(bufio.NewReader(resp.Body))
		if fetchErr != nil {
			return fetchErr
		}

		// For application/octet-stream, detect the actual image type from magic bytes.
		if contentType == "application/octet-stream" {
			contentType = http.DetectContentType(body)
		}

		filename, fetchErr = appendImageExtension(contentType, filenameNoExt)
		if fetchErr != nil {
			return retry.Unrecoverable(fetchErr)
		}

		return nil
	}); err != nil {
		if errors.Is(err, sharedhttp.ErrNotFound) {
			log.Warn().Msgf("image url returned 404, skipping %q", url)
			return nil
		}

		return err
	}

	// Decrypt if needed.
	if xorKeyHex != "" {
		key, err := hex.DecodeString(xorKeyHex)
		if err != nil {
			return fmt.Errorf("decoding encryption key: %w", err)
		}

		for i := range body {
			body[i] ^= key[i%len(key)]
		}
	}

	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", filename, err)
	}
	defer out.Close()

	_, err = io.Copy(out, bytes.NewReader(body))
	return err
}

// fetchWithRetry executes the GET request with a common retry strategy and passes the successful response to onSuccess.
func fetchWithRetry(ctx context.Context, url string, onSuccess func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return onSuccess(&resp)
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second),
		retry.Context(ctx),
	)
}

// appendImageExtension returns filename with an extension deduced from the content-type header.
func appendImageExtension(contentType, filename string) (string, error) {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return filename + ".jpg", nil
	case "image/png":
		return filename + ".png", nil
	case "image/gif":
		return filename + ".gif", nil
	case "image/webp":
		return filename + ".webp", nil
	default:
		return filename, fmt.Errorf("unsupported content type: %s", contentType)
	}
}
