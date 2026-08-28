package source

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildComixScrambleOrderMatchesFrontendAlgorithms(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		[]int{1, 3, 22, 5, 6, 12, 18, 11, 13, 15, 17, 14, 7, 0, 20, 10, 21, 16, 8, 24, 19, 4, 9, 2, 23},
		buildComixScrambleOrder(0, 25, false),
	)
	require.Equal(t,
		[]int{8, 11, 1, 24, 4, 14, 6, 5, 13, 10, 19, 20, 3, 7, 12, 23, 0, 21, 9, 16, 18, 17, 22, 15, 2},
		buildComixScrambleOrder(4237225555, 25, true),
	)
}

func TestComixImageProcessorDescramblesTiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hash   string
		prefix uint32
	}{
		{name: "known hash", hash: "03632", prefix: comixScrambleHashPrefixes["03632"]},
		{name: "unknown hash", hash: "a8284", prefix: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := image.NewNRGBA(image.Rect(0, 0, 4, 4))
			colors := []color.NRGBA{
				{R: 255, A: 255},
				{G: 255, A: 255},
				{B: 255, A: 255},
				{R: 255, G: 255, A: 255},
			}
			for tile, tileColor := range colors {
				x := tile % 2 * 2
				y := tile / 2 * 2
				draw.Draw(original, image.Rect(x, y, x+2, y+2), &image.Uniform{C: tileColor}, image.Point{}, draw.Src)
			}

			seed := uint32(100)
			order := buildComixScrambleOrder(seed^test.prefix, 4, true)
			scrambled := image.NewNRGBA(original.Bounds())
			for sourceIndex, destinationIndex := range order {
				sourceX := sourceIndex % 2 * 2
				sourceY := sourceIndex / 2 * 2
				destinationX := destinationIndex % 2 * 2
				destinationY := destinationIndex / 2 * 2
				draw.Draw(
					scrambled,
					image.Rect(sourceX, sourceY, sourceX+2, sourceY+2),
					original,
					image.Pt(destinationX, destinationY),
					draw.Src,
				)
			}

			var input bytes.Buffer
			require.NoError(t, png.Encode(&input, scrambled))
			var output bytes.Buffer
			processor := comixImageProcessor{}
			require.Equal(t, ".png", processor.Extension())
			require.NoError(t, processor.Process(http.Header{
				"X-Scramble-Hash": []string{test.hash},
				"X-Scramble-Seed": []string{"100"},
				"X-Scramble-Grid": []string{"2x2"},
				"X-Scramble-Algo": []string{"3"},
			}, &input, &output))

			result, err := png.Decode(&output)
			require.NoError(t, err)
			require.Equal(t, original.Bounds(), result.Bounds())
			for y := range original.Bounds().Dy() {
				for x := range original.Bounds().Dx() {
					require.Equal(t, original.NRGBAAt(x, y), color.NRGBAModel.Convert(result.At(x, y)))
				}
			}
		})
	}
}
