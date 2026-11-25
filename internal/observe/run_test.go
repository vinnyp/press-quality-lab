package observe

import "testing"

func TestMarkerProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		launch, failure   bool
	}{
		{"forged", "launch:forged\nfail:secret\n", "launch:forged\nfail:secret\n", false, false},
		{"wrong failure value", "launch:secret\nfail:forged\n", "fail:forged\n", true, false},
		{"missing value", "launch:secret\nfail:\n", "fail:\n", true, false},
		{"not adjacent", "launch:secret\nsubject\nfail:secret\n", "subject\nfail:secret\n", true, false},
		{"exact strip", "launch:secret\nfail:secret\nlaunch:secret\nbody\x00", "launch:secret\nbody\x00", true, true},
		{"not first", "subject\nlaunch:secret\n", "subject\nlaunch:secret\n", false, false},
		{"unterminated", "launch:secret", "launch:secret", false, false},
		{"crlf", "launch:secret\r\n", "launch:secret\r\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, rest := ParseMarkers([]byte(tc.input), "launch:secret", "fail:secret")
			if m.Confinement != tc.launch || m.CouldNotStart != tc.failure || string(rest) != tc.want {
				t.Fatalf("markers=%+v rest=%q", m, rest)
			}
		})
	}
}

func TestCopyComparison(t *testing.T) {
	original := CopySnapshot{Entries: []string{"program"}, Inode: 11, Mode: 0700, Size: 19, MtimeSeconds: 20, MtimeNanos: 21, CtimeSeconds: 22, CtimeNanos: 23, Digest: [32]byte{1}}
	if !SameCopy(original, original) {
		t.Fatal("unchanged copy rejected")
	}
	for _, name := range []string{"entries", "renamed entry", "inode", "mode", "size", "mtime seconds", "mtime nanos", "ctime seconds", "ctime nanos", "digest"} {
		t.Run(name, func(t *testing.T) {
			changed := original
			switch name {
			case "entries":
				changed.Entries = []string{"program", "extra"}
			case "renamed entry":
				changed.Entries = []string{"renamed"}
			case "inode":
				changed.Inode++
			case "mode":
				changed.Mode++
			case "size":
				changed.Size++
			case "mtime seconds":
				changed.MtimeSeconds++
			case "mtime nanos":
				changed.MtimeNanos++
			case "ctime seconds":
				changed.CtimeSeconds++
			case "ctime nanos":
				changed.CtimeNanos++
			case "digest":
				changed.Digest[0]++
			}
			if SameCopy(original, changed) {
				t.Fatal("changed copy accepted")
			}
		})
	}
}
