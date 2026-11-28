package throwaway

import (
	"bytes"
	"errors"
	"io"
	"os"
)

const FileScanLimit int64 = 16 << 20
const TotalScanLimit int64 = 256 << 20

type ScanForm struct{ Name, Text string }

type ScanLeak struct{ Place, Encoding string }
type ScanResult struct {
	Leaks                    []ScanLeak
	BoundReached, Unreadable bool
}

// ScanHome scans only file contents, in byte-wise relative path order. Streams
// use QE10's kept-byte bound and do not consume the file budget.
// The caller supplies the shared encoder's forms in table order.
func ScanHome(home, planted string, forms []ScanForm, stdout, stderr []byte, cut bool) (ScanResult, error) {
	return scanHome(home, planted, forms, stdout, stderr, cut, openHomeFile, FileScanLimit, TotalScanLimit)
}
func scanHome(home, planted string, forms []ScanForm, stdout, stderr []byte, cut bool, open homeOpener, fileLimit, totalLimit int64) (out ScanResult, err error) {
	return scanHomeWith(home, planted, forms, stdout, stderr, cut, defaultHomeHooks(open), fileLimit, totalLimit)
}
func scanHomeWith(home, planted string, forms []ScanForm, stdout, stderr []byte, cut bool, hooks homeHooks, fileLimit, totalLimit int64) (out ScanResult, err error) {
	scan := func(place string, b []byte) {
		for _, f := range forms {
			if f.Text != "" && bytes.Contains(b, []byte(f.Text)) {
				out.Leaks = append(out.Leaks, ScanLeak{place, f.Name})
			}
		}
	}
	if len(forms) == 0 {
		return out, errors.New("missing leak scan forms")
	}
	for _, form := range forms {
		if form.Text == "" {
			return out, errors.New("empty leak scan form")
		}
	}
	const streamLimit = 1 << 20
	scan("stdout", stdout[:min(len(stdout), streamLimit)])
	scan("stderr", stderr[:min(len(stderr), streamLimit)])
	out.BoundReached = cut || len(stdout) > streamLimit || len(stderr) > streamLimit
	root, err := os.OpenRoot(home)
	if err != nil {
		out.Unreadable = true
		return out, nil
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	entries, walkErr := homeNamesWith(root, true, hooks)
	if walkErr != nil {
		out.BoundReached = out.BoundReached || homeBound(walkErr)
		out.Unreadable = !homeBound(walkErr)
	}
	deadline := hooks.now().Add(MaxWalkDuration)
	remaining := totalLimit
	for _, e := range entries {
		if !hooks.now().Before(deadline) {
			out.BoundReached = true
			break
		}
		if e.Kind != "file" || e.Path == planted {
			continue
		}
		limit := min(fileLimit, remaining)
		if e.Size > limit {
			out.BoundReached = true
		}
		if limit == 0 && e.Size > 0 {
			continue
		}
		readErr := withParent(root, e.Path, func(parent *os.Root, name string) error {
			f, err := checkedHomeFile(parent, name, hooks)
			if err != nil {
				return err
			}
			info, statErr := f.Stat()
			if statErr != nil || !os.SameFile(e.Info, info) {
				return errors.Join(errHomeUnreadable, statErr, f.Close())
			}
			b, readErr := io.ReadAll(io.LimitReader(deadlineReader{r: f, deadline: deadline, now: hooks.now}, limit))
			readErr = errors.Join(readErr, f.Close())
			remaining -= int64(len(b))
			scan("home/"+e.Path, b)
			if readErr == nil && !hooks.now().Before(deadline) {
				readErr = ErrTimeLimit
			}
			return readErr
		})
		if readErr != nil {
			out.Unreadable = true
			if errors.Is(readErr, ErrTimeLimit) {
				out.BoundReached = true
			}
		}
	}
	return out, nil
}
