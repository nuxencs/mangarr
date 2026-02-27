package download

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
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
	maxLoggedLinkLength         = 100
)

type ArchiveWriter func(log zerolog.Logger, tmpDir, outPath string, isManhwa bool) error

func Chapter(ctx context.Context, log zerolog.Logger, outputPath string, chapter domain.Chapter, isManhwa bool, archiveWriter ArchiveWriter) error {
	tmpDir, err := os.MkdirTemp("", "mangarr-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chapterLog := log.With().
		Float32("chapter_number", chapter.Number).
		Str("chapter_title", chapter.Title).
		Str("chapter_id", chapter.ID).
		Str("chapter_url", truncateLink(chapter.URL)).
		Int("image_total", len(chapter.ImageInfo)).
		Logger()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentImageDownloads)

	for i, img := range chapter.ImageInfo {
		g.Go(func() error {
			imageIndex := i + 1
			base := filepath.Join(tmpDir, fmt.Sprintf("%03d", imageIndex))
			imageLog := chapterLog.With().
				Int("image_index", imageIndex).
				Str("image_url", truncateLink(img.ImageURL)).
				Str("image_host", imageHost(img.ImageURL)).
				Bool("encrypted", img.EncryptionKey != "").
				Str("tmp_name", filepath.Base(base)).
				Logger()

			return downloadImage(ctx, imageLog, img.ImageURL, img.EncryptionKey, base, imageIndex, len(chapter.ImageInfo))
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
func downloadImage(ctx context.Context, log zerolog.Logger, imageURL, xorKeyHex, filenameNoExt string, imageIndex, imageTotal int) error {
	var (
		body        []byte
		contentType string
		filename    string
	)

	if err := fetchWithRetry(ctx, log, imageURL, func(resp *http.Response) error {
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
		return wrapImageError(imageIndex, imageTotal, imageURL, err)
	}

	// Decrypt if needed.
	if xorKeyHex != "" {
		key, err := hex.DecodeString(xorKeyHex)
		if err != nil {
			return wrapImageError(imageIndex, imageTotal, imageURL, fmt.Errorf("decoding encryption key: %w", err))
		}

		for i := range body {
			body[i] ^= key[i%len(key)]
		}
	}

	out, err := os.Create(filename)
	if err != nil {
		return wrapImageError(imageIndex, imageTotal, imageURL, fmt.Errorf("creating file %s: %w", filename, err))
	}
	defer out.Close()

	_, err = io.Copy(out, bytes.NewReader(body))
	if err != nil {
		return wrapImageError(imageIndex, imageTotal, imageURL, fmt.Errorf("writing file %s: %w", filename, err))
	}

	return nil
}

// fetchWithRetry executes the GET request with a common retry strategy and passes the successful response to onSuccess.
func fetchWithRetry(ctx context.Context, log zerolog.Logger, url string, onSuccess func(*http.Response) error) error {
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
		retry.OnRetry(func(n uint, err error) {
			log.Warn().
				Err(err).
				Uint("attempt_failed", n+1).
				Msg("Retrying image download")
		}),
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

func wrapImageError(imageIndex, imageTotal int, imageURL string, err error) error {
	return fmt.Errorf("image %d/%d (%s): %w", imageIndex, imageTotal, truncateLink(imageURL), err)
}

func imageHost(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}

	return parsed.Host
}

func truncateLink(link string) string {
	if len(link) <= maxLoggedLinkLength {
		return link
	}

	return link[:maxLoggedLinkLength] + "..."
}
