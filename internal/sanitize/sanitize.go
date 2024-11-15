package sanitize

import (
	"regexp"
	"strings"
)

// Filename removes problematic characters from the chapter title
func Filename(title string) string {
	// Compile the regex pattern
	r := regexp.MustCompile(`[<>:"/\\|?*]`)

	// Trim spaces & dots
	title = strings.Trim(title, " .")

	// Remove illegal chars
	title = r.ReplaceAllString(title, "")
	return title
}
