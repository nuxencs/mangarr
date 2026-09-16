package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"mangarr/internal/domain"

	"github.com/rs/zerolog"
)

// DownloadLogger keeps console diagnostics and a bounded, private run log separate.
// Close records the returned command error only in the file; main owns stderr errors.
type DownloadLogger struct {
	Log zerolog.Logger
	run *downloadLogFile
}

func NewDownload(cfg domain.Config, stderr io.Writer) (*DownloadLogger, error) {
	stderr = zerolog.SyncWriter(stderr)
	console := zerolog.ConsoleWriter{Out: stderr, TimeFormat: time.RFC3339}
	logging := &DownloadLogger{Log: zerolog.New(console).With().Timestamp().Logger()}
	if cfg.LogPath == "" {
		return logging, nil
	}

	level, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		return nil, fmt.Errorf("download log level: %w", err)
	}
	run, err := openDownloadLog(cfg.LogPath, cfg.LogMaxSize, cfg.LogMaxBackups)
	if err != nil {
		return nil, fmt.Errorf("initializing download logging beside %q: %w", cfg.LogPath, err)
	}
	run.level, run.stderr = level, stderr
	logging.run = run

	// Force an initial write so full disks fail before any provider work.
	initial := zerolog.New(run).With().Timestamp().Logger()
	initial.Log().Str("command", "download").Str("version", cfg.Version).Msg("Download run log")
	run.err = errors.Join(run.err, run.file.Sync())
	if run.err != nil {
		return nil, logging.Close(nil)
	}
	fmt.Fprintf(stderr, "Download log: %s\n", run.file.Name())
	// Retain diagnostics even if the exec session's stderr closes.
	logging.Log = zerolog.New(zerolog.MultiLevelWriter(run, console)).With().Timestamp().Logger()
	return logging, nil
}

func (l *DownloadLogger) Close(commandErr error) error {
	if l.run == nil {
		return commandErr
	}
	if commandErr != nil {
		fileLog := zerolog.New(l.run).With().Timestamp().Logger()
		fileLog.Error().Err(commandErr).Msg("Download failed")
	}
	if err := l.run.close(); err != nil {
		return errors.Join(commandErr, fmt.Errorf("download log %q is incomplete: %w", l.run.file.Name(), err))
	}
	return commandErr
}

const logLimitRecord = "{\"level\":\"error\",\"message\":\"Download log size limit reached; further diagnostics remain on stderr\"}\n"

var (
	errLogLimit = errors.New("logMaxSize reached; further diagnostics remain on stderr")
	logURL      = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s<>"]+`)
)

type downloadLogFile struct {
	mu       sync.Mutex
	file     *os.File
	stderr   io.Writer
	level    zerolog.Level
	maxBytes int64
	written  int64
	keep     int
	err      error
}

func (f *downloadLogFile) Write(p []byte) (int, error) {
	return f.WriteLevel(zerolog.NoLevel, p)
}

func (f *downloadLogFile) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil || (level != zerolog.NoLevel && level < f.level) {
		return len(p), nil
	}

	data, err := redactLogRecord(p)
	if err == nil {
		if int64(len(data))+f.written+int64(len(logLimitRecord)) > f.maxBytes {
			_, writeErr := f.file.WriteString(logLimitRecord)
			err = errors.Join(errLogLimit, writeErr)
		} else {
			var n int
			n, err = f.file.Write(data)
			f.written += int64(n)
			if err == nil && n != len(data) {
				err = io.ErrShortWrite
			}
		}
	}
	if err != nil {
		f.err = err
		fmt.Fprintf(f.stderr, "File logging failed for %q: %v\n", f.file.Name(), err)
	}
	// Keep acquisition and stderr running; Close returns the first file failure.
	return len(p), nil
}

func (f *downloadLogFile) close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = errors.Join(f.err, f.file.Sync(), f.file.Close())
	if err := pruneDownloadLogs(f.file.Name(), f.keep); err != nil {
		f.err = errors.Join(f.err, fmt.Errorf("retaining download logs: %w", err))
	}
	return f.err
}

func redactLogRecord(p []byte) ([]byte, error) {
	var record map[string]any
	decoder := json.NewDecoder(bytes.NewReader(p))
	decoder.UseNumber()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("decoding diagnostic record: %w", err)
	}
	redactLogValue(record)
	data, err := json.Marshal(record)
	return append(data, '\n'), err
}

func redactLogValue(value any) any {
	switch value := value.(type) {
	case string:
		return logURL.ReplaceAllStringFunc(value, func(raw string) string {
			parsed, err := url.Parse(raw)
			if err != nil {
				return "[redacted URL]"
			}
			parsed.User = nil
			parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
			parsed.ForceQuery = false
			return parsed.String()
		})
	case map[string]any:
		for key, child := range value {
			value[key] = redactLogValue(child)
		}
	case []any:
		for i, child := range value {
			value[i] = redactLogValue(child)
		}
	}
	return value
}
