package download

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
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
		Str("chapter_number", chapter.Number.String()).
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
				Bool("processed", img.Processor != nil).
				Str("tmp_name", filepath.Base(base)).
				Logger()

			return downloadImage(ctx, imageLog, img, base, imageIndex, len(chapter.ImageInfo))
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

// downloadImage fetches an image, applies its required transform, and stores it with the right extension.
func downloadImage(ctx context.Context, log zerolog.Logger, imageInfo domain.ImageInfo, filenameNoExt string, imageIndex, imageTotal int) error {
	var (
		filename string
		key      []byte
	)

	if imageInfo.EncryptionKey != "" {
		decodedKey, err := hex.DecodeString(imageInfo.EncryptionKey)
		if err != nil {
			return wrapImageError(imageIndex, imageTotal, imageInfo.ImageURL, fmt.Errorf("decoding encryption key: %w", err))
		}

		key = decodedKey
	}

	if err := fetchWithRetry(ctx, log, imageInfo.ImageURL, imageInfo.RequestHeaders, func(resp *http.Response) error {
		reader := bufio.NewReader(resp.Body)
		var src io.Reader = reader
		if len(key) > 0 {
			src = &xorReader{
				r:   reader,
				key: key,
			}
		}

		if imageInfo.Processor != nil {
			filename = filenameNoExt + imageInfo.Processor.Extension()
			if err := writeImageFile(filename, func(out io.Writer) error {
				return imageInfo.Processor.Process(resp.Header, src, out)
			}); err != nil {
				return retry.Unrecoverable(err)
			}

			return nil
		}

		contentType, err := detectContentType(resp, reader)
		if err != nil {
			return err
		}

		filename, err = appendImageExtension(contentType, filenameNoExt)
		if err != nil {
			return retry.Unrecoverable(err)
		}

		if err := writeImageFile(filename, func(out io.Writer) error {
			_, err := io.Copy(out, src)
			return err
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return wrapImageError(imageIndex, imageTotal, imageInfo.ImageURL, err)
	}

	return nil
}

func writeImageFile(filename string, write func(io.Writer) error) error {
	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", filename, err)
	}

	if err := write(out); err != nil {
		_ = out.Close()
		return fmt.Errorf("writing file %s: %w", filename, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing file %s: %w", filename, err)
	}

	return nil
}

func detectContentType(resp *http.Response, reader *bufio.Reader) (string, error) {
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err == nil {
			contentType = mediaType
		}
	}

	if contentType != "" && contentType != "application/octet-stream" {
		return contentType, nil
	}

	sniff, err := reader.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	return http.DetectContentType(sniff), nil
}

type xorReader struct {
	r   io.Reader
	key []byte
	pos int
}

func (r *xorReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	for i := range n {
		p[i] ^= r.key[r.pos%len(r.key)]
		r.pos++
	}

	return n, err
}

// fetchWithRetry executes the GET request with a common retry strategy and passes the successful response to onSuccess.
func fetchWithRetry(ctx context.Context, log zerolog.Logger, url string, headers map[string]string, onSuccess func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	for name, value := range headers {
		req.Header.Set(name, value)
	}

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
		sharedhttp.RetryOptions(ctx, retry.OnRetry(func(n uint, err error) {
			log.Warn().
				Err(err).
				Uint("attempt_failed", n+1).
				Msg("Retrying image download")
		}))...,
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
