package utils

import (
	"strconv"
	"strings"
)

// PadFloat formats a float32 with specified total width, preserving original decimals
func PadFloat(num float32, width int) string {
	// Convert float to string with full precision
	str := strconv.FormatFloat(float64(num), 'f', -1, 32)

	// Split into integer and decimal parts
	parts := strings.Split(str, ".")
	intPart := parts[0]

	// Calculate required padding for integer part only
	padding := width - len(intPart)

	// Add padding if needed
	if padding > 0 {
		intPart = strings.Repeat("0", padding) + intPart
	}

	// Reconstruct number with original decimal part if it exists
	if len(parts) > 1 {
		return intPart + "." + parts[1]
	}
	return intPart
}
