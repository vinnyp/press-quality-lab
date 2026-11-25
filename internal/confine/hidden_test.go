package confine

import (
	"encoding/hex"
	"reflect"
	"testing"
)

func TestLauncherArguments(t *testing.T) {
	got, err := LauncherArgs("marker", "program", []string{"--help", "__pql_fast", "a\"b"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"__pql_launcher", "marker", "program", `["--help","__pql_fast","a\"b"]`}
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
