package domain

import (
	"context"
	"io"
	"net/http"
)

type Source interface {
	String() string
	ValidateInput() error
	Discover(context.Context) (Manga, error)
	Pages(context.Context, Chapter) ([]ImageInfo, error)
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
	ImageURL       string
	EncryptionKey  string
	RequestHeaders map[string]string
	Processor      ImageProcessor
}

type ImageProcessor interface {
	Extension() string
	Process(http.Header, io.Reader, io.Writer) error
}
