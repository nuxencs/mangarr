package logger

import (
	"io"
	"os"
	"time"

	"mangarr/internal/domain"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

// DefaultLogger default logging controller
type DefaultLogger struct {
	log     zerolog.Logger
	level   zerolog.Level
	writers []io.Writer
}

func New(cfg *domain.Config) *DefaultLogger {
	l := &DefaultLogger{
		writers: make([]io.Writer, 0),
		level:   zerolog.DebugLevel,
	}

	// set log level
	l.SetLogLevel(cfg.LogLevel)

	// use pretty logging for dev only
	if cfg.Version == "dev" {
		// setup console writer
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

		l.writers = append(l.writers, consoleWriter)
	} else {
		// default to stderr
		l.writers = append(l.writers, os.Stderr)
	}

	if cfg.LogPath != "" {
		l.writers = append(l.writers,
			&lumberjack.Logger{
				Filename:   cfg.LogPath,
				MaxSize:    cfg.LogMaxSize, // megabytes
				MaxBackups: cfg.LogMaxBackups,
			},
		)
	}

	// set some defaults
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	// init new logger
	l.log = zerolog.New(io.MultiWriter(l.writers...)).With().Stack().Logger()

	return l
}

func (l *DefaultLogger) SetLogLevel(level string) {
	switch level {
	case "INFO":
		l.level = zerolog.InfoLevel
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "DEBUG":
		l.level = zerolog.DebugLevel
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "ERROR":
		l.level = zerolog.ErrorLevel
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "WARN":
		l.level = zerolog.WarnLevel
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "TRACE":
		l.level = zerolog.TraceLevel
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	default:
		l.level = zerolog.Disabled
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}
}

// Error log something at Error level
func (l *DefaultLogger) Error() *zerolog.Event {
	return l.log.Error().Timestamp()
}

// Info logs at info level.
func (l *DefaultLogger) Info() *zerolog.Event {
	return l.log.Info().Timestamp()
}

// Debug log something at debug level.
func (l *DefaultLogger) Debug() *zerolog.Event {
	return l.log.Debug().Timestamp()
}

// With log with context
func (l *DefaultLogger) With() zerolog.Context {
	return l.log.With().Timestamp()
}
