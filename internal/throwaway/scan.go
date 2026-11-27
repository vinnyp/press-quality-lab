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
		return out, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	entries, err := homeNames(root, true)
	if err != nil {
		return out, err
	}
	remaining := totalLimit
	for _, e := range entries {
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
		if root.Chmod(e.Path, 0600) != nil {
			out.Unreadable = true
			continue
		}
		f, err := open(root, e.Path)
		if err != nil {
			out.Unreadable = true
			continue
		}
		b, readErr := io.ReadAll(io.LimitReader(f, limit))
		readErr = errors.Join(readErr, f.Close())
		remaining -= int64(len(b))
		scan("home/"+e.Path, b)
		if readErr != nil {
			out.Unreadable = true
		}
	}
	return out, nil
}
