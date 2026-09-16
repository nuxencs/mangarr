package logger

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var downloadLogName = regexp.MustCompile(`^download-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z-[0-9a-f]{32}\.jsonl$`)

func openDownloadLog(logPath string, maxMiB, keep int) (*downloadLogFile, error) {
	if maxMiB <= 0 || int64(maxMiB) > (1<<63-1)/(1024*1024) || keep <= 0 {
		return nil, fmt.Errorf("logMaxSize and logMaxBackups must be positive and logMaxSize must fit in bytes")
	}
	if strings.HasSuffix(logPath, "/") || strings.HasSuffix(logPath, string(os.PathSeparator)) {
		return nil, fmt.Errorf("logPath must name a file, not a directory")
	}
	if info, err := os.Stat(logPath); err == nil && info.IsDir() {
		return nil, fmt.Errorf("logPath %q is a directory", logPath)
	}
	dir := logPath + ".downloads"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	guard, err := lockDownloadDirectory(dir)
	if err != nil {
		return nil, err
	}
	defer guard.Close()
	if err := pruneCompletedLogs(dir, keep); err != nil {
		return nil, err
	}

	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("generating run-log name: %w", err)
	}
	name := "download-" + time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(id[:]) + ".jsonl"
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := lockFile(file, false); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	return &downloadLogFile{file: file, maxBytes: int64(maxMiB) * 1024 * 1024, keep: keep}, nil
}

func lockDownloadDirectory(dir string) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := lockFile(file, false); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func pruneDownloadLogs(logPath string, keep int) error {
	dir := filepath.Dir(logPath)
	guard, err := lockDownloadDirectory(dir)
	if err != nil {
		return err
	}
	defer guard.Close()
	return pruneCompletedLogs(dir, keep)
}

// The directory lock covers creation until the run lock is held, and all cleanup.
func pruneCompletedLogs(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	slices.Reverse(entries) // Generated names sort by start time, newest first.
	completed := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !downloadLogName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		locked, lockErr := lockFile(file, true)
		closeErr := file.Close()
		if err := errors.Join(lockErr, closeErr); err != nil {
			return err
		}
		if !locked {
			continue
		}
		completed++
		if completed > keep {
			// Closed run names are never reopened by a writer; close before unlink for Windows.
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}
