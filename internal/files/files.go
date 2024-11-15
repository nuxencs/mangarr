package files

import (
	"archive/zip"
	"bufio"
	"image"
	"io"
	"os"
	"path/filepath"

	"github.com/go-pdf/fpdf"
	_ "golang.org/x/image/webp" // needed to decode webp
)

const binSize = 10

func IsValidLocation(location string) error {
	if _, err := os.Stat(location); err != nil {
		return err
	}

	return nil
}

// CreateCbzArchive creates a zip archive named cbzPath and adds all files from sourceDir to it
func CreateCbzArchive(sourceDir, cbzPath string, isManhwa bool) error {
	err := os.MkdirAll(filepath.Dir(cbzPath), os.ModePerm)
	if err != nil {
		return err
	}

	cbzFile, err := os.Create(cbzPath)
	if err != nil {
		return err
	}
	defer cbzFile.Close()

	writeBuf := bufio.NewWriter(cbzFile)
	defer writeBuf.Flush()

	zipWriter := zip.NewWriter(writeBuf)
	defer zipWriter.Close()

	var mostCommonWidth int
	widthCount := make(map[int]int)

	walkErr := filepath.Walk(sourceDir, func(imgPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		imgFile, err := os.Open(imgPath)
		if err != nil {
			return err
		}
		defer imgFile.Close()

		img, _, err := image.DecodeConfig(imgFile)
		if err != nil {
			return nil
		}

		bin := (img.Width / binSize) * binSize
		widthCount[bin]++

		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	maxCount := 0
	for bin, count := range widthCount {
		if count > maxCount {
			maxCount = count
			mostCommonWidth = bin
		}
	}

	walkErr = filepath.Walk(sourceDir, func(imgPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		imgFile, err := os.Open(imgPath)
		if err != nil {
			return err
		}
		defer imgFile.Close()

		img, _, err := image.DecodeConfig(imgFile)
		if err != nil {
			return nil
		}

		// only remove uncommon image widths for manhwa
		if isManhwa {
			if img.Width < mostCommonWidth-binSize || img.Width > mostCommonWidth+binSize {
				return nil
			}
		}

		return addFileToZip(zipWriter, imgPath, info.Name())
	})

	return walkErr
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

// addFileToZip adds a single file to the zip archive
func addFileToZip(zipWriter *zip.Writer, filePath, fileName string) error {
	fileToZip, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	writer, err := zipWriter.Create(fileName)
	if err != nil {
		return err
	}

	readerBuf := bufio.NewReader(fileToZip)

	_, err = io.Copy(writer, readerBuf)
	return err
}
