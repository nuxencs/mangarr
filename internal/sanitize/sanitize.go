package sanitize

import (
	"strings"
	"unicode"
)

var (
	illegalCharacters = map[rune]struct{}{
		'<':  {},
		'>':  {},
		':':  {},
		'/':  {},
		'\\': {},
		'|':  {},
		'?':  {},
		'*':  {},
	}
	replacementCharacters = map[rune]rune{
		'’': '\'', // fix inconsistency in naming between MangaDex and Flame Comics
	}
)

// Filename removes problematic characters and replaces specified characters in filename candidates
func Filename(title string) string {
	if len(title) == 0 {
		return ""
	}

	builder := new(strings.Builder)
	builder.Grow(len(title))

	for _, r := range title {
		// Skip illegal characters
		if _, illegal := illegalCharacters[r]; illegal {
			continue
		}

		// Use replacement if found
		if replacement, hasReplacement := replacementCharacters[r]; hasReplacement {
			builder.WriteRune(replacement)
		}

		// Keep the character as is
		builder.WriteRune(r)
	}

	sanitizedTitle := strings.TrimFunc(builder.String(), func(r rune) bool {
		return unicode.IsSpace(r) || r == '.'
	})

	return sanitizedTitle
}
