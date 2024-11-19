package sanitize

import (
	"regexp"
	"strings"
)

var (
	illegalCharacters = regexp.MustCompile(`[<>:"/\\|?*]`)
	replacementMap    = map[rune]rune{
		'’': '\'', // inconsistency in naming between MangaDex and Flame Comics
	}
)

// Filename removes problematic characters from the chapter title
func Filename(title string) string {
	// Trim spaces & dots
	title = strings.Trim(title, " .")

	// Remove illegal chars
	title = illegalCharacters.ReplaceAllString(title, "")

	// Replace characters based on the replacement map
	var result strings.Builder
	for _, char := range title {
		if replacement, ok := replacementMap[char]; ok {
			result.WriteRune(replacement) // Use replacement if found
		} else {
			result.WriteRune(char) // Keep the character as is
		}
	}

	return result.String()
}
