package files

import (
	"archive/zip"
	"bufio"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-pdf/fpdf"
	_ "golang.org/x/image/webp" // needed to decode webp
)

const binSize = 10

type imageMeta struct {
	path   string
	name   string
	width  int
	height int
}

func IsValidLocation(location string) error {
	if _, err := os.Stat(location); err != nil {
		return fmt.Errorf("stat location %s: %w", location, err)
	}

	return nil
}

// CreateCbzArchive creates a zip (.cbz) archive from the images in sourceDir.
func CreateCbzArchive(sourceDir, cbzPath string, isManhwa bool) error {
	if err := os.MkdirAll(filepath.Dir(cbzPath), os.ModePerm); err != nil {
		return fmt.Errorf("creating destination dir: %w", err)
	}

	var (
		images      []imageMeta
		widthCount  = make(map[int]int)
		mostCommonW int
	)

	if err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close()

		img, _, err := image.DecodeConfig(bufio.NewReader(f))
		if err != nil {
			return fmt.Errorf("decoding %s: %w", path, err)
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
	}); err != nil {
		return fmt.Errorf("scanning %s: %w", sourceDir, err)
	}

	// Determine the most common width bin.
	for bin, count := range widthCount {
		if count > widthCount[mostCommonW] {
			mostCommonW = bin
		}
	}

	// Sort images lexicographically so they stay in page order.
	sort.Slice(images, func(i, j int) bool { return images[i].name < images[j].name })

	cbzFile, err := os.Create(cbzPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", cbzPath, err)
	}
	defer cbzFile.Close()

	zipWriter := zip.NewWriter(bufio.NewWriter(cbzFile))
	defer zipWriter.Close()

	for _, img := range images {
		// TODO: find better solution for filtering
		// Skip pages that are highly likely not a Manhwa page
		if isManhwa {
			if img.width < mostCommonW-binSize || img.width > mostCommonW+binSize || img.width > img.height {
				// continue
			}
		}
		if err := addFileToZip(zipWriter, img.path, img.name); err != nil {
			return err
		}
	}

	return nil
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
