package confine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// HiddenPrefix reserves the first argument namespace before command parsing.
const HiddenPrefix = "__pql_"
const LauncherMode = "__pql_launcher"
const FastMode = "__pql_fast"
const MarkerPrefix = "pql-launch:"
const FailurePrefix = "pql-could-not-start:"

// CouldNotStart is reserved for launcher replacement failure, never attribution by itself.
const CouldNotStart = 72

// LauncherFailed reports a launcher infrastructure failure.
const LauncherFailed = 70

// NewMarker is called by the engine, never by the confined launcher.
func NewMarker() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// LauncherArgs has five fixed positional fields: mode, marker, executable,
// JSON program arguments (excluding argv[0]), and JSON program environment.
// The launcher runtime receives only EngineEnvironment, never subject values.
func LauncherArgs(marker, executable string, words, environment []string) ([]string, error) {
	for _, word := range append(append([]string{executable}, words...), environment...) {
		if strings.ContainsRune(word, 0) || !utf8.ValidString(word) {
			return nil, fmt.Errorf("invalid launcher word")
		}
	}
	b, err := json.Marshal(words)
	if err != nil {
		return nil, err
	}
	e, err := json.Marshal(environment)
	if err != nil {
		return nil, err
	}
	return []string{LauncherMode, marker, executable, string(b), string(e)}, nil
}

// EngineEnvironment isolates runtime initialisation from subject configuration.
func EngineEnvironment() []string { return []string{} }
