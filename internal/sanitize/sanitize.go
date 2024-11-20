package sanitize

import (
	"strings"
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
	// Process characters
	result := new(strings.Builder)
	for _, char := range title {
		if _, isIllegal := illegalCharacters[char]; isIllegal {
			// Skip illegal characters
			continue
		}
		if replacementCharacter, hasReplacement := replacementCharacters[char]; hasReplacement {
			// Use replacementCharacter if found
			result.WriteRune(replacementCharacter)
		} else {
			// Keep the character as is
			result.WriteRune(char)
		}
	}

	sanitizedTitle := strings.Trim(result.String(), " .")

	return sanitizedTitle
}
