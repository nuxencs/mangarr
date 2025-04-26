package utils

import (
	"strconv"
	"strings"
)

// PadFloat left-pads the integer portion of num with zeros so that it
// reaches the requested width, while keeping the original fractional part.
// Example: PadFloat(3.14, 4) => "0003.14".
func PadFloat(num float32, width int) string {
	// Convert with full precision for a float32.
	s := strconv.FormatFloat(float64(num), 'f', -1, 32)

	// Locate the decimal point (if any). Everything before it is the integer part.
	dot := strings.IndexByte(s, '.')
	if dot == -1 { // no fractional part
		dot = len(s)
	}

	// If the integer part is already at least 'width' wide, we can return early.
	if pad := width - dot; pad > 0 {
		var b strings.Builder
		b.Grow(width + len(s) - dot) // pre-allocate target size
		b.WriteString(strings.Repeat("0", pad))
		b.WriteString(s)
		return b.String()
	}

	return s
}
