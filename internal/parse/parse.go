package parse

import (
	"cmp"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"mangarr/internal/domain"
)

// ChapterSelection parses a comma-separated list that may contain single chapters or ranges (e.g. "1,2-4,5.5").
func ChapterSelection(input string, available map[float32]domain.Chapter) ([]float32, error) {
	uniq := make(map[float32]struct{})

	for _, raw := range strings.Split(input, ",") {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue // ignore empty segments like ",,"
		}

		// Range e.g. "2-5"
		if strings.ContainsRune(part, '-') {
			start, end, err := parseRange(part)
			if err != nil {
				return nil, err
			}
			for ch := range available {
				if ch >= start && ch <= end {
					uniq[ch] = struct{}{}
				}
			}
			continue
		}

		// Single chapter
		ch, err := parseChapter(part)
		if err != nil {
			return nil, err
		}
		uniq[ch] = struct{}{}
	}

	// Convert map → slice and sort for stable output.
	out := make([]float32, 0, len(uniq))
	for ch := range uniq {
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out, nil
}

// parseRange expects the form "start-end".
func parseRange(s string) (float32, float32, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q", s)
	}

	start, err := parseChapter(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("range start: %w", err)
	}
	end, err := parseChapter(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("range end: %w", err)
	}
	if start > end {
		return 0, 0, fmt.Errorf("start (%v) > end (%v)", start, end)
	}
	return start, end, nil
}

// parseChapter converts a trimmed string to float32.
func parseChapter(s string) (float32, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return 0, fmt.Errorf("invalid chapter %q: %w", s, err)
	}
	return float32(f), nil
}

// MinMaxKeys returns the lowest and highest keys from a map that has keys that can be ordered
func MinMaxKeys[K cmp.Ordered, V any](m map[K]V) (min K, max K, err error) {
	if len(m) == 0 {
		return min, max, fmt.Errorf("map is empty")
	}

	first := true
	for k := range m {
		if first {
			min, max = k, k
			first = false
			continue
		}

		if k < min {
			min = k
		}

		if k > max {
			max = k
		}
	}

	return min, max, nil
}
