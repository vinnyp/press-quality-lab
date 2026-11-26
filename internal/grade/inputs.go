package grade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	in.Input = plan.Input{Subject: s, Baseline: o.Baseline != "", Anchor: o.Anchor, Files: map[string][]byte{}}
	for _, token := range o.Scope {
		in.Scopes = append(in.Scopes, rulebook.Scope(token))
	}
	root, err := os.OpenRoot(filepath.Dir(o.SubjectFile))
	if err != nil {
		return in, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	open := func(name string) (*os.File, error) { return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) }

	read := func(name string) ([]byte, error) {
		f, e := openRegular(root, name, open)
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(io.LimitReader(f, (16<<20)+1))
		if len(b) > 16<<20 {
			e = errors.Join(e, errors.New("file exceeds 16 MiB"))
		}
		return b, errors.Join(e, f.Close())
	}
	if binary, ok := s["binary"].(string); ok {
		f, e := openRegular(root, binary, open)
		if e != nil {
			return in, failure(70, "QS8", "named file cannot be safely read")
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		e = errors.Join(e, f.Close())
		if e != nil {
			return in, failure(70, "QS8", "named file cannot be safely read")
		}
		digest := fmt.Sprintf("%x", h.Sum(nil))
		in.BinaryDigest = &digest
	}
	if seed, ok := s["seed"].(string); ok {
		info, e := root.Lstat(seed)
		if e != nil || !info.IsDir() {
			return in, failure(70, "QS9", "seed directory cannot be safely read")
		}
		err = fs.WalkDir(root.FS(), seed, func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return errors.New("seed contains a link or special file")
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
			var data []byte
			if !entry.IsDir() {
				data, e = read(path)
				if e != nil {
					return e
				}
			}
			in.Seed = append(in.Seed, plan.Entry{Path: filepath.ToSlash(rel), Directory: entry.IsDir(), Data: data})
			return nil
		})
		if err != nil {
			return in, failure(70, "QS9", "seed cannot be safely read")
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
					data, e := read(name)
					if e != nil {
						return failure(70, "QS8", "named file cannot be safely read")
					}
					in.Files[filepath.ToSlash(filepath.Clean(name))] = data
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
	if filepath.IsAbs(name) {
		return nil, errors.New("absolute named path")
	}
	info, e := root.Stat(name)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("regular file required")
	}
	f, e := open(name)
	if e != nil {
		return nil, e
	}
	info, e = f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(e, errors.New("regular file required"), f.Close())
	}
	return f, nil
}
