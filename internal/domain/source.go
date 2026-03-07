package domain

import (
	"context"
)

type Source interface {
	String() string
	ValidateInput() error
	GetManga(context.Context) (Manga, error)
	GetChapters(context.Context, Manga) error
	GetImageURLs(context.Context, *Chapter) error
}

type Manga struct {
	ID       string
	URL      string
	Title    string
	Chapters map[ChapterNumber]Chapter
	IsManhwa bool
}

type Chapter struct {
	ID        string
	URL       string
	Number    ChapterNumber
	Title     string
	ImageInfo []ImageInfo
}

type ImageInfo struct {
	ImageURL      string
	EncryptionKey string
}
