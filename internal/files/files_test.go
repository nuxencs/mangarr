package files

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestCreateCbzArchiveFlushesOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	imgPath := filepath.Join(sourceDir, "001.png")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}

	outPath := filepath.Join(tmpDir, "out.cbz")
	if err := CreateCbzArchive(zerolog.Nop(), sourceDir, outPath, false); err != nil {
		t.Fatalf("create cbz: %v", err)
	}

	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("open cbz: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file in cbz, got %d", len(r.File))
	}
}
