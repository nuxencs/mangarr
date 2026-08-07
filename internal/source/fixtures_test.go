package source

import (
	"embed"
	"testing"
)

//go:embed testdata/*
var sourceFixtures embed.FS

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := sourceFixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return contents
}
