package files

import (
	"archive/zip"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
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

func TestCreateCbzArchiveDoesNotPublishEmptyArchive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "empty")
	if err := os.Mkdir(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir empty source: %v", err)
	}
	outPath := filepath.Join(tmpDir, "out.cbz")
	err := CreateCbzArchive(zerolog.Nop(), sourceDir, outPath, false)

	if err == nil {
		t.Fatal("expected empty source directory to fail")
	}
	if _, statErr := os.Stat(outPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final archive exists after failure: %v", statErr)
	}
}

func TestPublishFileAtomicallyRemovesPartialOutput(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "chapter.cbz")
	wantErr := errors.New("simulated write failure")
	err := publishFileAtomically(destination, func(writer io.Writer) error {
		if _, err := writer.Write([]byte("partial archive")); err != nil {
			return err
		}
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("publish error = %v, want %v", err, wantErr)
	}
	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final file exists after failure: %v", statErr)
	}

	matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(destination), ".chapter.cbz.tmp-*"))
	if globErr != nil {
		t.Fatalf("glob temporary files: %v", globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after failure: %v", matches)
	}
}

func TestIsValidLocationRejectsFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := IsValidLocation(path); err == nil {
		t.Fatal("expected file location to fail validation")
	}
}
