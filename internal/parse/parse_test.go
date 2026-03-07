package parse

import (
	"testing"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/assert"
)

func chapterNumber(input string) domain.ChapterNumber {
	return domain.MustParseChapterNumber(input)
}

func chapterMap(inputs ...string) map[domain.ChapterNumber]domain.Chapter {
	chapters := make(map[domain.ChapterNumber]domain.Chapter, len(inputs))
	for _, input := range inputs {
		number := chapterNumber(input)
		chapters[number] = domain.Chapter{Number: number}
	}

	return chapters
}

func TestChapterSelection(t *testing.T) {
	availableChapters := chapterMap("1", "1.5", "2", "3", "4", "5", "10", "10.5", "1.01", "1.1")

	type args struct {
		input     string
		available map[domain.ChapterNumber]domain.Chapter
	}

	tests := []struct {
		name    string
		args    args
		want    []domain.ChapterNumber
		wantErr bool
	}{
		{
			name: "single_chapter",
			args: args{
				input:     "3.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("3")},
		},
		{
			name: "single_chapter_with_decimal",
			args: args{
				input:     "1.5",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1.5")},
		},
		{
			name: "multiple_single_chapters",
			args: args{
				input:     "1.0,3.0,5.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("3"), chapterNumber("5")},
		},
		{
			name: "simple_range",
			args: args{
				input:     "2.0-4.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("2"), chapterNumber("3"), chapterNumber("4")},
		},
		{
			name: "range_with_decimal",
			args: args{
				input:     "1.0-2.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{
				chapterNumber("1"),
				chapterNumber("1.01"),
				chapterNumber("1.1"),
				chapterNumber("1.5"),
				chapterNumber("2"),
			},
		},
		{
			name: "range_including_unavailable_chapters",
			args: args{
				input:     "4.0-7.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("4"), chapterNumber("5")},
		},
		{
			name: "single_chapters_and_range",
			args: args{
				input:     "1.0,3.0-5.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("3"), chapterNumber("4"), chapterNumber("5")},
		},
		{
			name: "multiple_ranges",
			args: args{
				input:     "1.0-2.0,4.0-5.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{
				chapterNumber("1"),
				chapterNumber("1.01"),
				chapterNumber("1.1"),
				chapterNumber("1.5"),
				chapterNumber("2"),
				chapterNumber("4"),
				chapterNumber("5"),
			},
		},
		{
			name: "complex_mix_with_whitespace",
			args: args{
				input:     " 1.0, 3.0 - 4.0 , 10.0-10.5",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("3"), chapterNumber("4"), chapterNumber("10"), chapterNumber("10.5")},
		},
		{
			name: "empty_input",
			args: args{
				input:     "",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{},
		},
		{
			name: "only_whitespace",
			args: args{
				input:     "   ",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{},
		},
		{
			name: "empty_segments",
			args: args{
				input:     "1.0,,3.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("3")},
		},
		{
			name: "duplicates",
			args: args{
				input:     "1.0,1.0,2.0-3.0,3.0",
				available: availableChapters,
			},
			want: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("2"), chapterNumber("3")},
		},
		{
			name: "empty_available_chapters",
			args: args{
				input:     "1.0-5.0",
				available: map[domain.ChapterNumber]domain.Chapter{},
			},
			want: []domain.ChapterNumber{},
		},
		{
			name: "invalid_chapter_format",
			args: args{
				input:     "abc",
				available: availableChapters,
			},
			wantErr: true,
		},
		{
			name: "invalid_range_format",
			args: args{
				input:     "1.0-2.0-3.0",
				available: availableChapters,
			},
			wantErr: true,
		},
		{
			name: "range_with_invalid_start",
			args: args{
				input:     "abc-2.0",
				available: availableChapters,
			},
			wantErr: true,
		},
		{
			name: "range_with_invalid_end",
			args: args{
				input:     "1.0-xyz",
				available: availableChapters,
			},
			wantErr: true,
		},
		{
			name: "start_greater_than_end",
			args: args{
				input:     "5.0-1.0",
				available: availableChapters,
			},
			wantErr: true,
		},
		{
			name: "mixed_valid_and_invalid",
			args: args{
				input:     "1.0,abc,3.0",
				available: availableChapters,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChapterSelection(tt.args.input, tt.args.available)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStart  domain.ChapterNumber
		wantEnd    domain.ChapterNumber
		wantErr    bool
		errMessage string
	}{
		{
			name:      "simple_range",
			input:     "1-3",
			wantStart: chapterNumber("1"),
			wantEnd:   chapterNumber("3"),
		},
		{
			name:      "range_with_decimals",
			input:     "1.5-3.7",
			wantStart: chapterNumber("1.5"),
			wantEnd:   chapterNumber("3.7"),
		},
		{
			name:      "range_with_whitespace",
			input:     " 1 - 3 ",
			wantStart: chapterNumber("1"),
			wantEnd:   chapterNumber("3"),
		},
		{
			name:       "invalid_range_format",
			input:      "1-2-3",
			wantErr:    true,
			errMessage: "invalid range",
		},
		{
			name:       "invalid_start_value",
			input:      "abc-3",
			wantErr:    true,
			errMessage: "range start",
		},
		{
			name:       "invalid_end_value",
			input:      "1-xyz",
			wantErr:    true,
			errMessage: "range end",
		},
		{
			name:       "start_greater_than_end",
			input:      "5-2",
			wantErr:    true,
			errMessage: "start (5) > end (2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseRange(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantStart, start)
			assert.Equal(t, tt.wantEnd, end)
		})
	}
}

func TestParseChapter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      domain.ChapterNumber
		wantErr   bool
		errPrefix string
	}{
		{
			name:  "integer",
			input: "5",
			want:  chapterNumber("5"),
		},
		{
			name:  "decimal",
			input: "3.5",
			want:  chapterNumber("3.5"),
		},
		{
			name:  "with_whitespace",
			input: " 2.5 ",
			want:  chapterNumber("2.5"),
		},
		{
			name:      "non_numeric_input",
			input:     "abc",
			wantErr:   true,
			errPrefix: "invalid chapter",
		},
		{
			name:      "mixed_numeric_and_non_numeric",
			input:     "1a",
			wantErr:   true,
			errPrefix: "invalid chapter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChapter(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errPrefix != "" {
					assert.Contains(t, err.Error(), tt.errPrefix)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMinMaxChapterNumbers(t *testing.T) {
	min, max, err := MinMaxChapterNumbers(chapterMap("10.5", "1", "1.01", "5"))
	assert.NoError(t, err)
	assert.Equal(t, chapterNumber("1"), min)
	assert.Equal(t, chapterNumber("10.5"), max)
}

func TestFormatChapterList(t *testing.T) {
	cases := []struct {
		name     string
		chapters []domain.ChapterNumber
		expected string
	}{
		{
			name:     "empty slice",
			chapters: nil,
			expected: "",
		},
		{
			name:     "single entry",
			chapters: []domain.ChapterNumber{chapterNumber("3")},
			expected: "3",
		},
		{
			name:     "unordered list",
			chapters: []domain.ChapterNumber{chapterNumber("5"), chapterNumber("1"), chapterNumber("3")},
			expected: "1, 3, 5",
		},
		{
			name: "decimal chapters",
			chapters: []domain.ChapterNumber{
				chapterNumber("1.5"),
				chapterNumber("1"),
				chapterNumber("2"),
				chapterNumber("3.5"),
				chapterNumber("3"),
			},
			expected: "1, 1.5, 2-3, 3.5",
		},
		{
			name: "duplicates",
			chapters: []domain.ChapterNumber{
				chapterNumber("1"),
				chapterNumber("1"),
				chapterNumber("2"),
				chapterNumber("3"),
				chapterNumber("3"),
			},
			expected: "1-3",
		},
		{
			name:     "non consecutive",
			chapters: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("3"), chapterNumber("5")},
			expected: "1, 3, 5",
		},
		{
			name:     "single gaps",
			chapters: []domain.ChapterNumber{chapterNumber("1"), chapterNumber("2"), chapterNumber("4"), chapterNumber("5"), chapterNumber("7")},
			expected: "1-2, 4-5, 7",
		},
		{
			name:     "decimal ordering preserves scale",
			chapters: []domain.ChapterNumber{chapterNumber("1.1"), chapterNumber("1.01"), chapterNumber("2")},
			expected: "1.01, 1.1, 2",
		},
	}

	for _, tc := range cases {
		ftc := tc
		t.Run(ftc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ftc.expected, FormatChapterList(ftc.chapters))
		})
	}
}
