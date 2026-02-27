package perf

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/rs/zerolog"
)

const shutdownTimeout = 5 * time.Second

type Logger interface {
	Error() *zerolog.Event
}

// StartPprofServer starts a dedicated pprof HTTP server.
func StartPprofServer(ctx context.Context, log Logger, address string) (string, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("error shutting down pprof server")
		}
	}()

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("pprof server stopped unexpectedly")
		}
	}()

	return listener.Addr().String(), nil
}
