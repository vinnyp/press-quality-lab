package confine

import (
	"encoding/hex"
	"reflect"
	"testing"
)

func TestLauncherArguments(t *testing.T) {
	got, err := LauncherArgs("marker", "program", []string{"--help", "__pql_fast", "a\"b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"__pql_launcher", "marker", "program", `["--help","__pql_fast","a\"b"]`, `null`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
func TestNewMarker(t *testing.T) {
	first, err := NewMarker()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewMarker()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(first)
	if err != nil || len(decoded) != 32 || first == second {
		t.Fatal("marker entropy/encoding differs")
	}
}

func TestLauncherInvalidWords(t *testing.T) {
	for _, word := range []string{"nul\x00word", string([]byte{0xff})} {
		if _, err := LauncherArgs("marker", "program", []string{word}, nil); err == nil {
			t.Fatal("invalid program word accepted")
		}
		if _, err := LauncherArgs("marker", "program", nil, []string{word}); err == nil {
			t.Fatal("invalid environment accepted")
		}
	}
	if len(EngineEnvironment()) != 0 {
		t.Fatal("launcher environment is not empty")
	}
	got, err := LauncherArgs("marker", "program", nil, []string{"GODEBUG=inittrace=1"})
	if err != nil || got[4] != `["GODEBUG=inittrace=1"]` {
		t.Fatalf("environment protocol: %q %v", got, err)
	}
}
