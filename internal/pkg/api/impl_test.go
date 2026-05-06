package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLuhnValid(t *testing.T) {
	require.True(t, luhnValid("79927398713"))
	require.False(t, luhnValid("79927398714"))
}

func TestIsDigitsOnly(t *testing.T) {
	require.True(t, isDigitsOnly("012345"))
	require.False(t, isDigitsOnly("12a34"))
	require.False(t, isDigitsOnly(""))
}

