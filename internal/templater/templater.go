package templater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mangarr/internal/domain"
	"mangarr/internal/utils"
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
		return fmt.Sprintf("%g", t.Chapter.Number)
	}

	length, _ := strconv.ParseInt(strings.ReplaceAll(options, ":", ""), 10, 32)
	return utils.PadFloat(t.Chapter.Number, int(length))
}

func (t *Templater) handleMangaTitle(options string) string {
	if t.Manga.Title == "" {
		return ""
	}

	cleanString := strings.ReplaceAll(options, ":", "")
	return strings.ReplaceAll(cleanString, "<.>", t.Manga.Title)
}

func (t *Templater) handleChapterTitle(options string) string {
	if t.Chapter.Title == "" {
		return ""
	}

	cleanString := strings.ReplaceAll(options, ":", "")
	return strings.ReplaceAll(cleanString, "<.>", t.Chapter.Title)
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
