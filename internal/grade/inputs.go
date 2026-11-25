package grade

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

// planningInput snapshots only the seed and declared computation inputs. It
// never opens a subject executable for execution, or creates a subject home.
func planningInput(o Options, s subject.File) (in plan.Input, err error) {
	in = plan.Input{Subject: s, Baseline: o.Baseline != "", Anchor: o.Anchor, Files: map[string][]byte{}}
	for _, token := range o.Scope {
		in.Scopes = append(in.Scopes, rulebook.Scope(token))
	}
	root, err := os.OpenRoot(filepath.Dir(o.SubjectFile))
	if err != nil {
		return in, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	read := func(name string) ([]byte, error) {
		f, e := root.Open(name)
		if e != nil {
			return nil, e
		}
		b, e := io.ReadAll(f)
		return b, errors.Join(e, f.Close())
	}
	if seed, ok := s["seed"].(string); ok {
		err = fs.WalkDir(root.FS(), seed, func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if path == seed {
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
			return in, err
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
						return e
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
