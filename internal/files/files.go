package files

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-pdf/fpdf"
	"github.com/rs/zerolog"
	_ "golang.org/x/image/webp" // needed to decode webp
)

const (
	binSize       = 10
	maxWidthMulti = 1.25

	// errUnsupportedSubsamplingRatio indicates an unsupported luma/chroma subsampling ratio in JPEG images.
	errUnsupportedSubsamplingRatio = jpeg.UnsupportedError("luma/chroma subsampling ratio")
)

type imageMeta struct {
	path   string
	name   string
	width  int
	height int
}

func IsValidLocation(location string) error {
	info, err := os.Stat(location)
	if err != nil {
		return fmt.Errorf("stat location %s: %w", location, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("location %s is not a directory", location)
	}

	return nil
}

// CreateCbzArchive creates a zip (.cbz) archive from the images in sourceDir.
func CreateCbzArchive(log zerolog.Logger, sourceDir, cbzPath string, isManhwa bool) error {
	if err := os.MkdirAll(filepath.Dir(cbzPath), os.ModePerm); err != nil {
		return fmt.Errorf("creating destination dir: %w", err)
	}

	var (
		images      []imageMeta
		widthCount  = make(map[int]int)
		mostCommonW int
	)

	if walkErr := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		f, openErr := os.Open(path)
		if openErr != nil {
			return fmt.Errorf("opening %s: %w", path, openErr)
		}
		defer f.Close()

		img, _, decodeErr := image.DecodeConfig(bufio.NewReader(f))
		if decodeErr != nil {
			if errors.Is(decodeErr, errUnsupportedSubsamplingRatio) {
				log.Debug().Str("cbz", filepath.Base(cbzPath)).Str("name", d.Name()).
					Msg("skipping size check for image because it has an unsupported subsampling ratio")

				images = append(images, imageMeta{
					path: path,
					name: d.Name(),
				})

				return nil
			}

			return fmt.Errorf("decoding %s: %w", path, decodeErr)
		}

		bin := (img.Width / binSize) * binSize
		widthCount[bin]++

		images = append(images, imageMeta{
			path:   path,
			name:   d.Name(),
			width:  img.Width,
			height: img.Height,
		})
		return nil
	}); walkErr != nil {
		return fmt.Errorf("walking directory %s: %w", sourceDir, walkErr)
	}

	// Determine the most common width bin.
	for bin, count := range widthCount {
		if count > widthCount[mostCommonW] {
			mostCommonW = bin
		}
	}

	// Sort images lexicographically so they stay in page order.
	sort.Slice(images, func(i, j int) bool { return images[i].name < images[j].name })

	selectedImages := make([]imageMeta, 0, len(images))
	for _, img := range images {
		// Skip pages that are highly likely not a Manhwa page
		if isManhwa && isLikelyUnwanted(img, mostCommonW) {
			log.Debug().Str("cbz", filepath.Base(cbzPath)).Str("name", img.name).
				Int("width", img.width).Int("height", img.height).
				Msg("skipped image because it's likely not a Manhwa page")
			continue
		}
		selectedImages = append(selectedImages, img)
	}
	if len(selectedImages) == 0 {
		return fmt.Errorf("creating archive: no images to write")
	}

	if err := publishFileAtomically(cbzPath, func(destination io.Writer) error {
		zipWriter := zip.NewWriter(destination)
		for _, img := range selectedImages {
			if err := addFileToZip(zipWriter, img.path, img.name); err != nil {
				_ = zipWriter.Close()
				return err
			}
		}

		if err := zipWriter.Close(); err != nil {
			return fmt.Errorf("closing zip archive: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("publishing %s: %w", cbzPath, err)
	}

	return nil
}

func publishFileAtomically(destinationPath string, write func(io.Writer) error) (err error) {
	destinationDir := filepath.Dir(destinationPath)
	tmpFile, err := os.CreateTemp(destinationDir, "."+filepath.Base(destinationPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}

	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if published {
			return
		}

		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0o644); err != nil {
		return fmt.Errorf("setting temporary file permissions: %w", err)
	}
	if err := write(tmpFile); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("syncing temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, destinationPath); err != nil {
		return fmt.Errorf("renaming temporary file: %w", err)
	}

	published = true
	return nil
}

func isLikelyUnwanted(img imageMeta, dominantW int) bool {
	// Skip pages that are not higher than wide
	if img.width < img.height {
		return false
	}

	// Skip pages that are wider than 1.25x the dominant width
	allowed := float64(dominantW) * maxWidthMulti
	if float64(img.width) < allowed {
		return false
	}

	return true
}

// CreatePDF creates a pdf file named pdfPath and adds all files from sourceDir to it
func CreatePDF(sourceDir, pdfPath string) error {
	err := os.MkdirAll(filepath.Dir(pdfPath), os.ModePerm)
	if err != nil {
		return err
	}

	pdf := fpdf.New(fpdf.OrientationPortrait, fpdf.UnitMillimeter, "", "")

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			pdfInfo := pdf.RegisterImageOptions(path, fpdf.ImageOptions{})
			imgWidth, imgHeight := pdfInfo.Extent()

			// filter out wide images
			if imgWidth > imgHeight {
				return nil
			}

			pdf.AddPageFormat(fpdf.OrientationPortrait, fpdf.SizeType{Wd: imgWidth, Ht: imgHeight})

			pdf.ImageOptions(path, 0, 0, imgWidth, imgHeight, false, fpdf.ImageOptions{}, 0, "")
		}

		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	return pdf.OutputFileAndClose(pdfPath)
}

// addFileToZip copies a single file into an open zip archive.
func addFileToZip(zipWriter *zip.Writer, filePath, fileName string) error {
	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filePath, err)
	}
	defer src.Close()

	hdr := &zip.FileHeader{
		Name:   fileName,
		Method: zip.Store,
	}
	dst, err := zipWriter.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("creating zip entry: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing %s: %w", fileName, err)
	}

	return nil
}
