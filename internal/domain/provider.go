package domain

import "context"

type Source interface {
	String() string
	ValidateInput() error
	GetManga(context.Context) (Manga, error)
	GetChapters(context.Context, Manga) error
	GetImageURLs(context.Context, *Chapter) error
}

type Manga struct {
	URL      string
	Title    string
	Chapters map[float32]Chapter
}

type Chapter struct {
	ID        string
	URL       string
	Number    float32
	Title     string
	IsManhwa  bool
	ImageInfo []ImageInfo
}

type ImageInfo struct {
	ImageURL      string
	EncryptionKey string
	Width         float64
	Height        float64
}
