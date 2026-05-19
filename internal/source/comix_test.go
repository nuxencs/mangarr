package source

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComixValidateInputDeprecated(t *testing.T) {
	t.Parallel()

	src := NewComix("https://comix.to/title/pvry-one-piece", "")

	err := src.ValidateInput()
	require.EqualError(t, err, comixDeprecatedError)
}
