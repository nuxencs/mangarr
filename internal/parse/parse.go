package parse

import (
	"fmt"
	"slices"
	"strings"

	"mangarr/internal/domain"
)

// ChapterSelection parses a comma-separated list that may contain single chapters or ranges (e.g. "1,2-4,5.5").
func ChapterSelection(input string, available map[domain.ChapterNumber]domain.Chapter) ([]domain.ChapterNumber, error) {
	uniq := make(map[domain.ChapterNumber]struct{})

	for raw := range strings.SplitSeq(input, ",") {
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
				if ch.Compare(start) >= 0 && ch.Compare(end) <= 0 {
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
	out := make([]domain.ChapterNumber, 0, len(uniq))
	for ch := range uniq {
		out = append(out, ch)
	}
	slices.SortFunc(out, func(a, b domain.ChapterNumber) int {
		return a.Compare(b)
	})

	return out, nil
}

// parseRange expects the form "start-end".
func parseRange(s string) (domain.ChapterNumber, domain.ChapterNumber, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return domain.ChapterNumber{}, domain.ChapterNumber{}, fmt.Errorf("invalid range %q", s)
	}

	start, err := parseChapter(parts[0])
	if err != nil {
		return domain.ChapterNumber{}, domain.ChapterNumber{}, fmt.Errorf("range start: %w", err)
	}
	end, err := parseChapter(parts[1])
	if err != nil {
		return domain.ChapterNumber{}, domain.ChapterNumber{}, fmt.Errorf("range end: %w", err)
	}
	if start.Compare(end) > 0 {
		return domain.ChapterNumber{}, domain.ChapterNumber{}, fmt.Errorf("start (%v) > end (%v)", start, end)
	}
	return start, end, nil
}

// parseChapter converts a trimmed string to ChapterNumber.
func parseChapter(s string) (domain.ChapterNumber, error) {
	number, err := domain.ParseChapterNumber(strings.TrimSpace(s))
	if err != nil {
		return domain.ChapterNumber{}, err
	}

	return number, nil
}

func MinMaxChapterNumbers(chapters map[domain.ChapterNumber]domain.Chapter) (domain.ChapterNumber, domain.ChapterNumber, error) {
	if len(chapters) == 0 {
		return domain.ChapterNumber{}, domain.ChapterNumber{}, fmt.Errorf("map is empty")
	}

	var (
		min   domain.ChapterNumber
		max   domain.ChapterNumber
		first = true
	)

	for number := range chapters {
		if first {
			min = number
			max = number
			first = false
			continue
		}

		if number.Less(min) {
			min = number
		}

		if max.Less(number) {
			max = number
		}
	}

	return min, max, nil
}

// FormatChapterList sorts the provided chapters and returns a string displaying
// consecutive numbers as ranges, e.g. "1-3, 5, 7.5".
func FormatChapterList(chapters []domain.ChapterNumber) string {
	if len(chapters) == 0 {
		return ""
	}

	sorted := slices.Clone(chapters)
	slices.SortFunc(sorted, func(a, b domain.ChapterNumber) int {
		return a.Compare(b)
	})

	var parts []string
	start := sorted[0]
	prev := sorted[0]

	for i := 1; i < len(sorted); i++ {
		current := sorted[i]
		if isConsecutive(prev, current) {
			prev = current
			continue
		}

		parts = append(parts, formatRange(start, prev))
		start = current
		prev = current
	}

	parts = append(parts, formatRange(start, prev))

	return strings.Join(parts, ", ")
}

func formatRange(start, end domain.ChapterNumber) string {
	if start.Equal(end) {
		return formatChapterNumber(start)
	}

	return fmt.Sprintf("%s-%s", formatChapterNumber(start), formatChapterNumber(end))
}

func formatChapterNumber(num domain.ChapterNumber) string {
	return num.String()
}

func isConsecutive(prev, current domain.ChapterNumber) bool {
	if prev.Equal(current) {
		return true
	}

	return !prev.HasFraction() && !current.HasFraction() && current.Whole == prev.Whole+1
}
