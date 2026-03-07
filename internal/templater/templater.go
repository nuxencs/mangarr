package templater

import (
	"regexp"
	"strconv"
	"strings"

	"mangarr/internal/domain"
)

var templatePattern = regexp.MustCompile(`{((\w+?)(:.*?)?)}`)

type Templater struct {
	Manga   domain.Manga
	Chapter domain.Chapter
}

func New(manga domain.Manga, chapter domain.Chapter) *Templater {
	return &Templater{
		Manga:   manga,
		Chapter: chapter,
	}
}

func (t *Templater) handleNum(options string) string {
	if options == "" {
		return t.Chapter.Number.String()
	}

	length, _ := strconv.ParseInt(strings.ReplaceAll(options, ":", ""), 10, 32)
	return t.Chapter.Number.Pad(int(length))
}

func (t *Templater) handleMangaTitle(options string) string {
	if t.Manga.Title == "" {
		return ""
	}

	clean := strings.ReplaceAll(options, ":", "")
	return strings.ReplaceAll(clean, "<.>", t.Manga.Title)
}

func (t *Templater) handleChapterTitle(options string) string {
	if t.Chapter.Title == "" {
		return ""
	}

	clean := strings.ReplaceAll(options, ":", "")
	return strings.ReplaceAll(clean, "<.>", t.Chapter.Title)
}

func (t *Templater) ExecTemplate(template string) string {
	newString := template
	for _, match := range templatePattern.FindAllStringSubmatch(template, -1) {
		replace := match[0]

		varName := match[2]
		switch varName {
		case "num":
			options := ""
			if len(match) > 3 {
				options = match[3]
			}
			replace = t.handleNum(options)
		case "manga":
			replace = t.handleMangaTitle(match[3])
		case "title":
			replace = t.handleChapterTitle(match[3])
		}

		newString = strings.Replace(newString, match[0], replace, 1)
	}

	return newString
}
