package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestVersionSucceedsWhenUpdateCheckFails(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	root := newRootCommand(dependencies{
		versionClient: client,
		releaseURL:    githubURL,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	if !strings.Contains(stdout.String(), "Version:") {
		t.Fatalf("version output = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Update check unavailable:") || !strings.Contains(stderr.String(), "offline") {
		t.Fatalf("update warning = %q", stderr.String())
	}
}

func TestMonitorReturnsConfigError(t *testing.T) {
	root := newRootCommand(defaultDependencies())
	root.SetArgs([]string{"monitor", "--config", t.TempDir()})

	if err := root.Execute(); err == nil {
		t.Fatal("expected invalid generated config to fail")
	}
}

func TestRootCommandsDoNotShareFlagState(t *testing.T) {
	first := newRootCommand(defaultDependencies())
	first.SetOut(io.Discard)
	first.SetArgs([]string{"download", "--help"})
	if err := first.Execute(); err != nil {
		t.Fatalf("execute first root: %v", err)
	}

	second := newRootCommand(defaultDependencies())
	flag := second.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("second root has no config flag")
	}
	if flag.Changed {
		t.Fatal("second root inherited changed flag state")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
