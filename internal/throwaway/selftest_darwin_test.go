package throwaway

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelfTestCalibration(t *testing.T) {
	// QH26: metadata observation does not access data; reading moves atime; replacement is refused.
	for _, name := range []string{"NoRead", "Read", "Replaced"} {
		t.Run(name, func(t *testing.T) {
			w, err := BeginAt(filepath.Join(t.TempDir(), "runs"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := w.Remove(); err != nil {
					t.Error(err)
				}
			}()
			f, err := w.PrepareSelfTest()
			if err != nil {
				t.Fatal(err)
			}
			after, err := f.CalibrationAfter()
			if err != nil || !after.Equal(f.Before) {
				t.Fatalf("no read: %v %v", after, err)
			}
			switch name {
			case "Read":
				if _, err := os.ReadFile(f.Calibration); err != nil {
					t.Fatal(err)
				}
			case "Replaced":
				replacement := filepath.Join(f.Home, "replacement")
				if err := os.WriteFile(replacement, []byte("calibration_value\n"), 0600); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(f.Calibration)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, f.Calibration); err != nil {
					t.Fatal(err)
				}
			}
			after, err = f.CalibrationAfter()
			if name == "Replaced" {
				if err == nil {
					t.Fatal("replacement accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name == "Read" && !after.After(f.Before) || name == "NoRead" && !after.Equal(f.Before) {
				t.Fatalf("atime before=%v after=%v", f.Before, after)
			}
		})
	}
}

func TestSelfTestDecoys(t *testing.T) {
	// QH9: a violator cannot pass by returning the expected raw kill.
	for _, name := range []string{"Outside", "Create", "OutsideLink"} {
		t.Run(name, func(t *testing.T) {
			w, err := BeginAt(filepath.Join(t.TempDir(), "runs"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := w.Remove(); err != nil {
					t.Error(err)
				}
			}()
			f, err := w.PrepareSelfTest()
			if err != nil {
				t.Fatal(err)
			}
			if err := f.CheckDecoys(); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "Outside":
				err = os.WriteFile(f.Outside, []byte("changed"), 0600)
			case "Create":
				err = os.WriteFile(f.Create, []byte("changed"), 0600)
			case "OutsideLink":
				link := filepath.Join(f.Home, "link")
				if err := os.Symlink(f.Calibration, link); err != nil {
					t.Fatal(err)
				}
				err = os.Rename(link, f.Outside)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := f.CheckDecoys(); err == nil {
				t.Fatal("changed decoy accepted")
			}
		})
	}
}
