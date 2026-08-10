package source

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
)

const (
	comixScrambleAlgorithmV1A = "1"
	comixScrambleAlgorithmV1B = "2"
	comixScrambleAlgorithmV2  = "3"

	comixLCGMultiplier uint32 = 1664525
	comixLCGIncrement  uint32 = 1013904223

	comixXorShiftLeftA = 13
	comixXorShiftRight = 17
	comixXorShiftLeftB = 5
)

// These values and shuffle versions match frontend build comixFrontendBuild.
var comixScrambleHashPrefixes = map[string]uint32{
	"03632": 58414,
	"02900": 117532,
}

type comixImageProcessor struct{}

func (comixImageProcessor) Extension() string {
	return ".png"
}

func (comixImageProcessor) Process(headers http.Header, reader io.Reader, writer io.Writer) error {
	source, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("decoding scrambled Comix image: %w", err)
	}

	result, err := descrambleComixImage(source, headers)
	if err != nil {
		return err
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(writer, result); err != nil {
		return fmt.Errorf("encoding descrambled Comix image: %w", err)
	}

	return nil
}

func descrambleComixImage(source image.Image, headers http.Header) (image.Image, error) {
	hash := strings.ToLower(headers.Get("X-Scramble-Hash"))
	prefix, ok := comixScrambleHashPrefixes[hash]
	if !ok {
		return nil, fmt.Errorf("descrambling Comix image: unsupported hash %q", hash)
	}

	seedValue := headers.Get("X-Scramble-Seed")
	seed, err := strconv.ParseUint(seedValue, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("descrambling Comix image: invalid seed %q: %w", seedValue, err)
	}
	columns, rows, err := parseComixScrambleGrid(headers.Get("X-Scramble-Grid"))
	if err != nil {
		return nil, err
	}

	algorithm := headers.Get("X-Scramble-Algo")
	if algorithm != comixScrambleAlgorithmV1A &&
		algorithm != comixScrambleAlgorithmV1B &&
		algorithm != comixScrambleAlgorithmV2 {
		return nil, fmt.Errorf("descrambling Comix image: unsupported algorithm %q", algorithm)
	}

	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	tileWidth := width / columns
	tileHeight := height / rows
	if tileWidth < 1 || tileHeight < 1 {
		return nil, fmt.Errorf("descrambling Comix image: grid %dx%d exceeds image size %dx%d", columns, rows, width, height)
	}

	order := buildComixScrambleOrder(uint32(seed)^prefix, columns*rows, algorithm == comixScrambleAlgorithmV2)
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for sourceIndex, destinationIndex := range order {
		sourceX := sourceIndex % columns * tileWidth
		sourceY := sourceIndex / columns * tileHeight
		destinationX := destinationIndex % columns * tileWidth
		destinationY := destinationIndex / columns * tileHeight
		destination := image.Rect(destinationX, destinationY, destinationX+tileWidth, destinationY+tileHeight)
		draw.Draw(result, destination, source, image.Pt(bounds.Min.X+sourceX, bounds.Min.Y+sourceY), draw.Src)
	}

	return result, nil
}

func parseComixScrambleGrid(value string) (int, int, error) {
	parts := strings.Split(value, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("descrambling Comix image: invalid grid %q", value)
	}

	columns, err := strconv.Atoi(parts[0])
	if err != nil || columns < 1 {
		return 0, 0, fmt.Errorf("descrambling Comix image: invalid grid %q", value)
	}
	rows, err := strconv.Atoi(parts[1])
	if err != nil || rows < 1 {
		return 0, 0, fmt.Errorf("descrambling Comix image: invalid grid %q", value)
	}

	return columns, rows, nil
}

func buildComixScrambleOrder(seed uint32, count int, versionTwo bool) []int {
	order := make([]int, count)
	for i := range order {
		order[i] = i
	}

	state := seed
	if versionTwo {
		state |= 1
	}
	for i := count; i >= 2; i-- {
		if versionTwo {
			state ^= state << comixXorShiftLeftA
			state ^= state >> comixXorShiftRight
			state ^= state << comixXorShiftLeftB
		} else {
			state = state*comixLCGMultiplier + comixLCGIncrement
		}
		other := int(state % uint32(i))
		order[i-1], order[other] = order[other], order[i-1]
	}

	return order
}
