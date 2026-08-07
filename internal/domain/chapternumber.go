package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ChapterNumber struct {
	Whole    int
	Fraction int
	Scale    int
}

var pow10Table = [...]int{
	1,
	10,
	100,
	1_000,
	10_000,
	100_000,
	1_000_000,
	10_000_000,
	100_000_000,
	1_000_000_000,
}

func ParseChapterNumber(input string) (ChapterNumber, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return ChapterNumber{}, fmt.Errorf("invalid chapter %q", input)
	}

	wholePart, fractionPart, hasDot := strings.Cut(s, ".")
	if wholePart == "" || !digitsOnly(wholePart) {
		return ChapterNumber{}, fmt.Errorf("invalid chapter %q", input)
	}

	whole, err := strconv.Atoi(wholePart)
	if err != nil {
		return ChapterNumber{}, fmt.Errorf("invalid chapter %q: %w", input, err)
	}

	if !hasDot {
		return ChapterNumber{Whole: whole}, nil
	}

	if fractionPart == "" || !digitsOnly(fractionPart) {
		return ChapterNumber{}, fmt.Errorf("invalid chapter %q", input)
	}

	fractionPart = strings.TrimRight(fractionPart, "0")
	if fractionPart == "" {
		return ChapterNumber{Whole: whole}, nil
	}

	fraction, err := strconv.Atoi(fractionPart)
	if err != nil {
		return ChapterNumber{}, fmt.Errorf("invalid chapter %q: %w", input, err)
	}

	return ChapterNumber{
		Whole:    whole,
		Fraction: fraction,
		Scale:    len(fractionPart),
	}, nil
}

func (c ChapterNumber) String() string {
	whole := strconv.Itoa(c.Whole)
	if !c.HasFraction() {
		return whole
	}

	fraction := fmt.Sprintf("%0*d", c.Scale, c.Fraction)
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return whole
	}

	return whole + "." + fraction
}

func (c ChapterNumber) Pad(width int) string {
	if width <= 0 {
		return c.String()
	}

	whole := strconv.Itoa(c.Whole)
	if len(whole) < width {
		whole = strings.Repeat("0", width-len(whole)) + whole
	}

	if !c.HasFraction() {
		return whole
	}

	return whole + "." + fmt.Sprintf("%0*d", c.Scale, c.Fraction)
}

func (c ChapterNumber) Compare(other ChapterNumber) int {
	switch {
	case c.Whole < other.Whole:
		return -1
	case c.Whole > other.Whole:
		return 1
	}

	scale := max(c.Scale, other.Scale)
	left := c.Fraction * pow10(scale-c.Scale)
	right := other.Fraction * pow10(scale-other.Scale)

	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func (c ChapterNumber) Less(other ChapterNumber) bool {
	return c.Compare(other) < 0
}

func (c ChapterNumber) Equal(other ChapterNumber) bool {
	return c.Compare(other) == 0
}

func (c ChapterNumber) IsZero() bool {
	return c.Whole == 0 && c.Fraction == 0
}

func (c ChapterNumber) HasFraction() bool {
	return c.Scale > 0
}

func (c *ChapterNumber) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" {
		*c = ChapterNumber{}
		return nil
	}

	if len(raw) == 0 {
		return fmt.Errorf("invalid chapter number JSON")
	}

	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}

		number, err := ParseChapterNumber(s)
		if err != nil {
			return err
		}

		*c = number
		return nil
	}

	number, err := ParseChapterNumber(raw)
	if err != nil {
		return err
	}

	*c = number
	return nil
}

func digitsOnly(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func pow10(exp int) int {
	if exp < len(pow10Table) {
		return pow10Table[exp]
	}

	value := pow10Table[len(pow10Table)-1]
	for i := len(pow10Table) - 1; i < exp; i++ {
		value *= 10
	}

	return value
}
