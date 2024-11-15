package download

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/files"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
)

// Chapter downloads and processes manga chapter images to create a CBZ archive.
func Chapter(ctx context.Context, contentPath string, chapter domain.Chapter) error {
	var wg sync.WaitGroup

	// if chapter.IsManhwa {
	// 	 outputPath = contentPath + ".pdf"
	// } else {
	// 	 outputPath = contentPath + ".cbz"
	// }

	temp, err := os.MkdirTemp("", "mangarr-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	for i, imageInfo := range chapter.ImageInfo {
		wg.Add(1)

		i, imageInfo := i, imageInfo

		go func() {
			defer wg.Done()

			filenameNoExt := filepath.Join(temp, fmt.Sprintf("%03d", i+1))

			if len(imageInfo.EncryptionKey) != 0 {
				if err = decryptImage(ctx, imageInfo.ImageURL, imageInfo.EncryptionKey, filenameNoExt); err != nil {
					fmt.Printf("error decrypting and downloading file: %q", err)
					return
				}
			} else {
				if err = singleFile(ctx, imageInfo.ImageURL, filenameNoExt); err != nil {
					fmt.Printf("error downloading file: %q", err)
					return
				}
			}
		}()
	}
	wg.Wait()

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

	if err := files.CreateCbzArchive(temp, contentPath, chapter.IsManhwa); err != nil {
		return err
	}

	return nil
}

// singleFile downloads a single file
func singleFile(ctx context.Context, url, filenameNoExt string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	retryErr := retry.Do(func() error {
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get image: %w", err)
		}
		defer resp.Body.Close()

		if err := sharedhttp.CheckStatusCode(resp.StatusCode); err != nil {
			return err
		}

		filename, err := appendImageExtension(resp, filenameNoExt)
		if err != nil {
			return err
		}

		out, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer out.Close()

		readBuf := bufio.NewReader(resp.Body)
		writeBuf := bufio.NewWriter(out)
		defer writeBuf.Flush()

		_, err = io.Copy(writeBuf, readBuf)
		if err != nil {
			return err
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)

	return retryErr
}

// decryptImage fetches an image from the URL and decrypts it with the given encryption key.
func decryptImage(ctx context.Context, url string, encryptionHex string, filenameNoExt string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	retryErr := retry.Do(func() error {
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get image: %w", err)
		}
		defer resp.Body.Close()

		if err := sharedhttp.CheckStatusCode(resp.StatusCode); err != nil {
			return err
		}

		data, err := io.ReadAll(bufio.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("failed to read image data: %w", err)
		}

		key, err := hex.DecodeString(encryptionHex)
		if err != nil {
			return fmt.Errorf("failed to decode encryption key: %w", err)
		}

		// perform XOR decryption
		keyLen := len(key)
		for i := range data {
			data[i] ^= key[i%keyLen]
		}

		filename, err := appendImageExtension(resp, filenameNoExt)
		if err != nil {
			return err
		}

		out, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer out.Close()

		byteBuf := bytes.NewBuffer(data)
		writeBuf := bufio.NewWriter(out)
		defer writeBuf.Flush()

		_, err = io.Copy(writeBuf, byteBuf)
		if err != nil {
			return err
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)

	return retryErr
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
