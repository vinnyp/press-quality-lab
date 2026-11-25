package confine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

// HiddenPrefix reserves the first argument namespace before command parsing.
const HiddenPrefix = "__pql_"
const LauncherMode = "__pql_launcher"
const FastMode = "__pql_fast"
const MarkerPrefix = "pql-launch:"
const FailurePrefix = "pql-could-not-start:"

// CouldNotStart is reserved for launcher replacement failure, never attribution by itself.
const CouldNotStart = 72

// NewMarker is called by the engine, never by the confined launcher.
func NewMarker() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// LauncherArgs has four fixed positional fields: mode, marker, executable,
// JSON array of program arguments (excluding argv[0]). No word is an option.
func LauncherArgs(marker, executable string, words []string) ([]string, error) {
	b, err := json.Marshal(words)
	if err != nil {
		return nil, err
	}
	return []string{LauncherMode, marker, executable, string(b)}, nil
}
