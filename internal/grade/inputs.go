package grade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

type snapshot struct {
	plan.Input
	BinaryDigest *string
}

// planningInput snapshots the binary, seed and declared computation inputs. It
// never opens a subject executable for execution, or creates a subject home.
func planningInput(o Options, s subject.File) (in snapshot, err error) {
	in.Input = plan.Input{Subject: s, Baseline: o.Baseline != "", Anchor: o.Anchor, FileDigests: map[string]string{}}
	for _, token := range o.Scope {
		in.Scopes = append(in.Scopes, rulebook.Scope(token))
	}
	root, err := os.OpenRoot(filepath.Dir(o.SubjectFile))
	if err != nil {
		return in, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	open := func(name string) (*os.File, error) { return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) }

	// Only digests enter the plan; streaming keeps seed and named-file sizes
	// out of the engine's memory budget without imposing a specification limit.
	digest := func(name string) (string, error) {
		f, e := openRegular(root, name, open)
		if e != nil {
			return "", e
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		e = errors.Join(e, f.Close())
		return fmt.Sprintf("%x", h.Sum(nil)), e
	}
	if binary, ok := s["binary"].(string); ok {
		sum, e := digest(binary)
		if e != nil {
			return in, safeReadFailure("QS8", "named file", e)
		}
		in.BinaryDigest = &sum
	}
	if seed, ok := s["seed"].(string); ok {
		// QS9's treatment of links naming the seed awaits U9. Fail closed
		// for every component, including links whose targets stay inside root.
		component := ""
		if filepath.IsAbs(seed) {
			return in, safeReadFailure("QS9", "seed directory", errEscape)
		}
		for _, part := range strings.Split(filepath.ToSlash(seed), "/") {
			if part == ".." && (component == "" || component == ".") {
				return in, safeReadFailure("QS9", "seed directory", errEscape)
			}
			component = filepath.Join(component, part)
			info, e := root.Lstat(component)
			if e != nil {
				return in, safeReadFailure("QS9", "seed directory", e)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return in, safeReadFailure("QS9", "seed directory", errLink)
			}
			if !info.IsDir() {
				return in, safeReadFailure("QS9", "seed directory", errSpecial)
			}
		}
		err = fs.WalkDir(root.FS(), seed, func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errLink
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return errSpecial
			}
			if path == seed {
				if !entry.IsDir() {
					return errors.New("seed directory required")
				}
				return nil
			}
			rel, e := filepath.Rel(seed, path)
			if e != nil {
				return e
			}
			var sum string
			if !entry.IsDir() {
				sum, e = digest(path)
				if e != nil {
					return e
				}
			}
			in.Seed = append(in.Seed, plan.Entry{Path: filepath.ToSlash(rel), Directory: entry.IsDir(), Digest: sum})
			return nil
		})
		if err != nil {
			return in, safeReadFailure("QS9", "seed", err)
		}
	}
	var visit func(any) error
	visit = func(v any) error {
		switch v := v.(type) {
		case strictjson.Object:
			if argv, ok := v["argv"].(strictjson.Array); ok {
				names, namesOK := v["declared_files"].(strictjson.Array)
				if !namesOK {
					names = strictjson.Array{}
				}
				names = append(strictjson.Array{}, names...)
				if len(argv) > 0 {
					if name, ok := argv[0].(string); ok && !filepath.IsAbs(name) {
						names = append(names, name)
					}
				}
				for _, n := range names {
					name, ok := n.(string)
					if !ok {
						continue
					}
					sum, e := digest(name)
					if e != nil {
						return safeReadFailure("QS8", "named file", e)
					}
					in.FileDigests[filepath.ToSlash(filepath.Clean(name))] = sum
				}
			}
			for _, child := range v {
				if e := visit(child); e != nil {
					return e
				}
			}
		case strictjson.Array:
			for _, child := range v {
				if e := visit(child); e != nil {
					return e
				}
			}
		}
		return nil
	}
	err = visit(s)
	return in, err
}

func openInput(path string) (io.ReadCloser, error) { return os.Open(path) }

// openRegular checks type before opening so static special files are never
// opened. The descriptor check also rejects a replacement during the open.
func openRegular(root *os.Root, name string, open func(string) (*os.File, error)) (*os.File, error) {
	if !filepath.IsLocal(name) {
		return nil, errEscape
	}
	info, e := root.Stat(name)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, errSpecial
	}
	f, e := open(name)
	if e != nil {
		return nil, e
	}
	info, e = f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(e, errSpecial, f.Close())
	}
	return f, nil
}

var (
	errEscape  = errors.New("escape")
	errLink    = errors.New("link")
	errSpecial = errors.New("special")
)

// safeReadFailure exposes only a category, never an underlying path or value.
func safeReadFailure(rule, kind string, err error) *Error {
	category := "special"
	switch {
	case errors.Is(err, errEscape):
		category = "escape"
	case errors.Is(err, os.ErrNotExist):
		category = "missing"
	case errors.Is(err, errLink), errors.Is(err, syscall.ELOOP):
		category = "link"
	}
	return failure(70, rule, kind+" cannot be safely read: "+category)
}
