package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMainReturnsFailureForInvalidDownloadDirectory(t *testing.T) {
	const (
		helperEnvironment    = "MANGARR_TEST_INVALID_DOWNLOAD_DIRECTORY"
		directoryEnvironment = "MANGARR_TEST_DOWNLOAD_DIRECTORY"
	)
	if os.Getenv(helperEnvironment) == "1" {
		os.Args = []string{
			"mangarr",
			"download", "--downloadDirectory", os.Getenv(directoryEnvironment),
			"--source", "comix",
			"--manga", "https://comix.to/title/pvry-one-piece",
		}
		main()
		return
	}

	missingDirectory := filepath.Join(t.TempDir(), "missing")
	command := exec.Command(os.Args[0], "-test.run=^TestMainReturnsFailureForInvalidDownloadDirectory$")
	command.Env = append(
		os.Environ(),
		helperEnvironment+"=1",
		directoryEnvironment+"="+missingDirectory,
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("mangarr exited successfully for an invalid download directory:\n%s", output)
	}

	exitError, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("running mangarr: %v", err)
	}
	if exitError.ExitCode() != 1 {
		t.Fatalf("mangarr exit code = %d, want 1:\n%s", exitError.ExitCode(), output)
	}
}
