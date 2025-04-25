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
	"sync"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/files"
	"mangarr/internal/semaphore"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"github.com/rs/zerolog"
)

const maxConcurrentImageDownloads = 10

// Chapter downloads and processes manga chapter images to create a CBZ archive.
func Chapter(ctx context.Context, log zerolog.Logger, contentPath string, chapter domain.Chapter, isManhwa bool) error {
	// if chapter.IsManhwa {
	// 	 outputPath = contentPath + ".pdf"
	// } else {
	// 	 outputPath = contentPath + ".cbz"
	// }

	temp, err := os.MkdirTemp("", "mangarr-*")
	if err != nil {
		return fmt.Errorf("creating temp directory %s: %w", temp, err)
	}
	defer os.RemoveAll(temp)

	// semaphore to limit concurrency to maxConcurrentImageDownloads which is set to 10
	sem := semaphore.NewWeighted(maxConcurrentImageDownloads)
	errc := make(chan error, len(chapter.ImageInfo))
	var wg sync.WaitGroup

	for i, imageInfo := range chapter.ImageInfo {
		wg.Add(1)

		go func() {
			sem.Acquire()
			defer func() { sem.Release(); wg.Done() }()

			filenameNoExt := filepath.Join(temp, fmt.Sprintf("%03d", i+1))

			var err error
			if len(imageInfo.EncryptionKey) != 0 {
				if err = decryptImage(ctx, log, imageInfo.ImageURL, imageInfo.EncryptionKey, filenameNoExt); err != nil {
					errc <- fmt.Errorf("decrypting and download image %s: %w", imageInfo.ImageURL, err)
					return
				}
			} else {
				if err = singleFile(ctx, log, imageInfo.ImageURL, filenameNoExt); err != nil {
					errc <- fmt.Errorf("downloading image %s: %w", imageInfo.ImageURL, err)
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	var errors []error
	for err := range errc {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("processing %d images: %w", len(errors), errors[0])
	}

	// if chapter.IsManhwa {
	// 	 err = files.CreatePDF(temp, outputPath)
	// 	 if err != nil {
	// 	 	 return err
	// 	 }
	// } else {
	// 	 err = files.CreateCbzArchive(temp, outputPath)
	// 	 if err != nil {
	// 	 	 return err
	// 	 }
	// }

	if err := files.CreateCbzArchive(temp, contentPath, isManhwa); err != nil {
		return fmt.Errorf("creating cbz archive: %w", err)
	}

	return nil
}

// singleFile downloads a single file
func singleFile(ctx context.Context, log zerolog.Logger, url, filenameNoExt string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(client, req)
		if err != nil {
			if errors.Is(err, sharedhttp.ErrNotFound) {
				log.Debug().Msgf("image not found, skipping %q", url)
				return nil
			}
			return fmt.Errorf("executing request: %w", err)
		}

		filename, err := appendImageExtension(&resp, filenameNoExt)
		if err != nil {
			return fmt.Errorf("appending image extension: %w", err)
		}

		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", filename, err)
		}
		defer out.Close()

		readBuf := bufio.NewReader(resp.Body)
		writeBuf := bufio.NewWriter(out)
		defer writeBuf.Flush()

		_, err = io.Copy(writeBuf, readBuf)
		if err != nil {
			return fmt.Errorf("copying buffer to file %s: %w", filename, err)
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return fmt.Errorf("executing request: %w", retryErr)
	}

	return nil
}

// decryptImage fetches an image from the URL and decrypts it with the given encryption key.
func decryptImage(ctx context.Context, log zerolog.Logger, url string, encryptionHex string, filenameNoExt string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(client, req)
		if err != nil {
			if errors.Is(err, sharedhttp.ErrNotFound) {
				log.Debug().Msgf("image not found, skipping %q", url)
				return nil
			}
			return fmt.Errorf("executing request: %w", err)
		}

		data, err := io.ReadAll(bufio.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		key, err := hex.DecodeString(encryptionHex)
		if err != nil {
			return fmt.Errorf("decoding image using encryption key: %w", err)
		}

		// perform XOR decryption
		keyLen := len(key)
		for i := range data {
			data[i] ^= key[i%keyLen]
		}

		filename, err := appendImageExtension(&resp, filenameNoExt)
		if err != nil {
			return fmt.Errorf("appending image extension: %w", err)
		}

		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", filename, err)
		}
		defer out.Close()

		byteBuf := bytes.NewBuffer(data)
		writeBuf := bufio.NewWriter(out)
		defer writeBuf.Flush()

		_, err = io.Copy(writeBuf, byteBuf)
		if err != nil {
			return fmt.Errorf("copying buffer to file %s: %w", filename, err)
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return fmt.Errorf("executing request: %w", retryErr)
	}

	return nil
}

func appendImageExtension(resp *http.Response, filename string) (string, error) {
	contentType := resp.Header.Get("Content-Type")

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
