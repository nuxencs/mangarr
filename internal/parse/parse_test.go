package parse

import (
	"testing"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestChapterSelection(t *testing.T) {
	// Create a test fixture with available chapters
	availableChapters := map[float32]domain.Chapter{
		1.0:  {},
		1.5:  {},
		2.0:  {},
		3.0:  {},
		4.0:  {},
		5.0:  {},
		10.0: {},
		10.5: {},
	}

	type args struct {
		input     string
		available map[float32]domain.Chapter
	}
	tests := []struct {
		name    string
		args    args
		want    []float32
		wantErr bool
	}{
		// Single chapter inputs
		{
			name: "single_chapter",
			args: args{
				input:     "3.0",
				available: availableChapters,
			},
			want:    []float32{3.0},
			wantErr: false,
		},
		{
			name: "single_chapter_with_decimal",
			args: args{
				input:     "1.5",
				available: availableChapters,
			},
			want:    []float32{1.5},
			wantErr: false,
		},
		{
			name: "multiple_single_chapters",
			args: args{
				input:     "1.0,3.0,5.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 3.0, 5.0},
			wantErr: false,
		},

		// Range inputs
		{
			name: "simple_range",
			args: args{
				input:     "2.0-4.0",
				available: availableChapters,
			},
			want:    []float32{2.0, 3.0, 4.0},
			wantErr: false,
		},
		{
			name: "range_with_decimal",
			args: args{
				input:     "1.0-2.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 1.5, 2.0},
			wantErr: false,
		},
		{
			name: "range_including_unavailable_chapters",
			args: args{
				input:     "4.0-7.0",
				available: availableChapters,
			},
			want:    []float32{4.0, 5.0},
			wantErr: false,
		},

		// Mixed inputs
		{
			name: "single_chapters_and_range",
			args: args{
				input:     "1.0,3.0-5.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 3.0, 4.0, 5.0},
			wantErr: false,
		},
		{
			name: "multiple_ranges",
			args: args{
				input:     "1.0-2.0,4.0-5.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 1.5, 2.0, 4.0, 5.0},
			wantErr: false,
		},
		{
			name: "complex_mix_with_whitespace",
			args: args{
				input:     " 1.0, 3.0 - 4.0 , 10.0-10.5",
				available: availableChapters,
			},
			want:    []float32{1.0, 3.0, 4.0, 10.0, 10.5},
			wantErr: false,
		},

		// Edge cases
		{
			name: "empty_input",
			args: args{
				input:     "",
				available: availableChapters,
			},
			want:    []float32{},
			wantErr: false,
		},
		{
			name: "only_whitespace",
			args: args{
				input:     "   ",
				available: availableChapters,
			},
			want:    []float32{},
			wantErr: false,
		},
		{
			name: "empty_segments",
			args: args{
				input:     "1.0,,3.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 3.0},
			wantErr: false,
		},
		{
			name: "duplicates",
			args: args{
				input:     "1.0,1.0,2.0-3.0,3.0",
				available: availableChapters,
			},
			want:    []float32{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name: "empty_available_chapters",
			args: args{
				input:     "1.0-5.0",
				available: map[float32]domain.Chapter{},
			},
			want:    []float32{},
			wantErr: false,
		},

		// Error cases
		{
			name: "invalid_chapter_format",
			args: args{
				input:     "abc",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid_range_format",
			args: args{
				input:     "1.0-2.0-3.0",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "range_with_invalid_start",
			args: args{
				input:     "abc-2.0",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "range_with_invalid_end",
			args: args{
				input:     "1.0-xyz",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "start_greater_than_end",
			args: args{
				input:     "5.0-1.0",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "mixed_valid_and_invalid",
			args: args{
				input:     "1.0,abc,3.0",
				available: availableChapters,
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChapterSelection(tt.args.input, tt.args.available)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStart  float32
		wantEnd    float32
		wantErr    bool
		errMessage string
	}{
		{
			name:      "simple_range",
			input:     "1-3",
			wantStart: 1.0,
			wantEnd:   3.0,
			wantErr:   false,
		},
		{
			name:      "range_with_decimals",
			input:     "1.5-3.7",
			wantStart: 1.5,
			wantEnd:   3.7,
			wantErr:   false,
		},
		{
			name:      "range_with_whitespace",
			input:     " 1 - 3 ",
			wantStart: 1.0,
			wantEnd:   3.0,
			wantErr:   false,
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
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			}
		})
	}
}

func TestParseChapter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      float32
		wantErr   bool
		errPrefix string
	}{
		{
			name:    "integer",
			input:   "5",
			want:    5.0,
			wantErr: false,
		},
		{
			name:    "decimal",
			input:   "3.5",
			want:    3.5,
			wantErr: false,
		},
		{
			name:    "with_whitespace",
			input:   " 2.5 ",
			want:    2.5,
			wantErr: false,
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
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
