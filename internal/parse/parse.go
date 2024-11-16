package parse

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"mangarr/internal/domain"
)

// ChapterSelection parses the user input for ranges and parts
func ChapterSelection(input string, availableChapters map[float32]domain.Chapter) ([]float32, error) {
	parts := strings.Split(input, ",")
	uniqueChapters := make(map[float32]bool)

	for _, part := range parts {
		if strings.Contains(part, "-") {
			start, end, err := getRange(part)
			if err != nil {
				return nil, fmt.Errorf("failed to get range from part %s: %w", part, err)
			}

			for chapter := range availableChapters {
				if chapter >= start && chapter <= end {
					uniqueChapters[chapter] = true
				}
			}
		} else {
			chapter, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse chapter number from part %s: %w", part, err)
			}

			uniqueChapters[float32(chapter)] = true
		}
	}

	selectedChapters := make([]float32, 0, len(uniqueChapters))
	for chapterNumber := range uniqueChapters {
		selectedChapters = append(selectedChapters, chapterNumber)
	}

	return selectedChapters, nil
}

// getRange parses the user input for chapter ranges
func getRange(part string) (float32, float32, error) {
	rangeParts := strings.Split(part, "-")
	if len(rangeParts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format: %s", part)
	}

	start, err := strconv.ParseFloat(strings.TrimSpace(rangeParts[0]), 32)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse start of range %s: %w", rangeParts[0], err)
	}

	end, err := strconv.ParseFloat(strings.TrimSpace(rangeParts[1]), 32)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse end of range %s: %w", rangeParts[1], err)
	}

	if start > end {
		return 0, 0, fmt.Errorf("start of range should not be greater than end")
	}

	return float32(start), float32(end), nil
}

// GetMinAndMaxKeys returns the lowest and highest keys from a map that has keys that can be ordered
func GetMinAndMaxKeys[K cmp.Ordered, V any](someMap map[K]V) ([]K, []K, error) {
	if len(someMap) == 0 {
		var zero []K
		return zero, zero, fmt.Errorf("map is empty")
	}

	keys := make([]K, 0, len(someMap))
	for key := range someMap {
		keys = append(keys, key)
	}

	return []K{slices.Min(keys)}, []K{slices.Max(keys)}, nil
}
