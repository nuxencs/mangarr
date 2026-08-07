package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseChapterNumber(input string) ChapterNumber {
	number, err := ParseChapterNumber(input)
	if err != nil {
		panic(err)
	}
	return number
}

func TestParseChapterNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ChapterNumber
		wantErr bool
	}{
		{
			name:  "integer",
			input: "12",
			want:  ChapterNumber{Whole: 12},
		},
		{
			name:  "decimal",
			input: "12.34",
			want:  ChapterNumber{Whole: 12, Fraction: 34, Scale: 2},
		},
		{
			name:  "trim trailing zeroes",
			input: "1.500",
			want:  ChapterNumber{Whole: 1, Fraction: 5, Scale: 1},
		},
		{
			name:  "leading zeroes",
			input: "001.0500",
			want:  ChapterNumber{Whole: 1, Fraction: 5, Scale: 2},
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "alpha suffix",
			input:   "12a",
			wantErr: true,
		},
		{
			name:    "missing whole part",
			input:   ".5",
			wantErr: true,
		},
		{
			name:    "missing fraction part",
			input:   "5.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChapterNumber(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChapterNumberString(t *testing.T) {
	tests := []struct {
		name   string
		number ChapterNumber
		want   string
	}{
		{
			name:   "whole",
			number: ChapterNumber{Whole: 1},
			want:   "1",
		},
		{
			name:   "decimal",
			number: ChapterNumber{Whole: 1, Fraction: 5, Scale: 1},
			want:   "1.5",
		},
		{
			name:   "keeps scale-significant zeroes",
			number: ChapterNumber{Whole: 1, Fraction: 1, Scale: 2},
			want:   "1.01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.number.String())
		})
	}
}

func TestChapterNumberPad(t *testing.T) {
	assert.Equal(t, "003", ChapterNumber{Whole: 3}.Pad(3))
	assert.Equal(t, "0003.14", ChapterNumber{Whole: 3, Fraction: 14, Scale: 2}.Pad(4))
	assert.Equal(t, "0010.01", ChapterNumber{Whole: 10, Fraction: 1, Scale: 2}.Pad(4))
}

func TestChapterNumberCompare(t *testing.T) {
	one := mustParseChapterNumber("1")
	onePointZeroOne := mustParseChapterNumber("1.01")
	onePointOne := mustParseChapterNumber("1.1")
	tenPointFive := mustParseChapterNumber("10.5")

	assert.True(t, one.Less(onePointZeroOne))
	assert.True(t, onePointZeroOne.Less(onePointOne))
	assert.True(t, onePointOne.Less(tenPointFive))
	assert.Equal(t, 0, mustParseChapterNumber("1.50").Compare(mustParseChapterNumber("1.5")))
}

func TestChapterNumberUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ChapterNumber
		wantErr bool
	}{
		{
			name:  "numeric json",
			input: `1.01`,
			want:  ChapterNumber{Whole: 1, Fraction: 1, Scale: 2},
		},
		{
			name:  "string json",
			input: `"10.5"`,
			want:  ChapterNumber{Whole: 10, Fraction: 5, Scale: 1},
		},
		{
			name:    "invalid json number",
			input:   `1e2`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ChapterNumber
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
