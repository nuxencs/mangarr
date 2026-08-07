package source

import (
	"embed"
	"testing"

	"mangarr/internal/domain"
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

func mustChapterNumber(input string) domain.ChapterNumber {
	number, err := domain.ParseChapterNumber(input)
	if err != nil {
		panic(err)
	}
	return number
}
