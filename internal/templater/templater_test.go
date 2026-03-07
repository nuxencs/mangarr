package templater

import (
	"testing"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestExecTemplateUsesCanonicalChapterNumber(t *testing.T) {
	tpl := New(
		domain.Manga{Title: "Series"},
		domain.Chapter{Number: domain.MustParseChapterNumber("1.50"), Title: "Pilot"},
	)

	assert.Equal(t, "Series Ch. 1.5 - Pilot", tpl.ExecTemplate("{manga:<.>} Ch. {num}{title: - <.>}"))
}

func TestExecTemplatePadsWholePart(t *testing.T) {
	tpl := New(
		domain.Manga{Title: "Series"},
		domain.Chapter{Number: domain.MustParseChapterNumber("10.01")},
	)

	assert.Equal(t, "0010.01", tpl.ExecTemplate("{num:4}"))
}
