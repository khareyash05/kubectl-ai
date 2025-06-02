package main

import (
	"testing"

	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveQueryInput_SingleArgWithStdin_002 tests the behavior of resolveQueryInput when a single positional argument is provided along with stdin data.
func TestResolveQueryInput_SingleArgWithStdin_002(t *testing.T) {
	// Arrange
	args := []string{"get"}
	hasStdInData := true
	stdinData := "pods\n"

	// Mock stdin
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.Write([]byte(stdinData))
	w.Close()

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "get\npods", query)
}

// TestResolveQueryInput_StdinOnly_003 tests the behavior of resolveQueryInput when only stdin data is provided.
func TestResolveQueryInput_StdinOnly_003(t *testing.T) {
	// Arrange
	args := []string{}
	hasStdInData := true
	stdinData := "get pods\n"

	// Mock stdin
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.Write([]byte(stdinData))
	w.Close()

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "get pods", query)
}

// TestResolveQueryInput_SingleArgNoStdin_001 tests the behavior of resolveQueryInput when a single positional argument is provided and no stdin data is available.
func TestResolveQueryInput_SingleArgNoStdin_001(t *testing.T) {
	// Arrange
	args := []string{"get pods"}
	hasStdInData := false

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "get pods", query)
}

// TestResolveQueryInput_SingleArgWithStdin_EmptyCombinedQuery_234 tests the scenario where
// a single argument and stdin data are provided, but their combination results in an empty query.
func TestResolveQueryInput_SingleArgWithStdin_EmptyCombinedQuery_234(t *testing.T) {
	// Arrange
	args := []string{"   "} // Argument is whitespace
	hasStdInData := true
	stdinData := " \n " // Stdin data is also whitespace

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.Write([]byte(stdinData))
	w.Close()

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no query provided from stdin")
	assert.Equal(t, "", query)
}

// TestResolveQueryInput_NoInput_004 tests the behavior of resolveQueryInput when no input is provided.
func TestResolveQueryInput_NoInput_004(t *testing.T) {
	// Arrange
	args := []string{}
	hasStdInData := false

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "", query)
}

// TestResolveQueryInput_StdinOnly_EmptyStdinQuery_890 tests the scenario where
// no arguments are provided, stdin data is whitespace, resulting in an empty query.
func TestResolveQueryInput_StdinOnly_EmptyStdinQuery_890(t *testing.T) {
	// Arrange
	args := []string{}
	hasStdInData := true
	stdinData := "   \n   " // Stdin data is whitespace

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.Write([]byte(stdinData))
	w.Close()

	// Act
	query, err := resolveQueryInput(hasStdInData, args)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no query provided from stdin")
	assert.Equal(t, "", query)
}
