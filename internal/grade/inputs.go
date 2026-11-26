package grade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
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
	directory, e := filepath.EvalSymlinks(filepath.Dir(o.SubjectFile))
	if e != nil {
		return in, safeReadFailure("QS8", "subject directory", e)
	}
	directory, e = filepath.Abs(directory)
	if e != nil {
		return in, e
	}
	account, e := user.Current()
	if e != nil {
		return in, e
	}
	home, e := filepath.EvalSymlinks(account.HomeDir)
	if e != nil {
		return in, e
	}
	if inside(directory, home) {
		return in, safeReadFailure("QS8", "subject directory", errEscape)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return in, safeReadFailure("QS8", "subject directory", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	resolve := func(name string) (string, error) { return resolveInput(directory, name) }
	open := func(name string) (*os.File, error) { return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) }

	// Only digests enter the plan; streaming keeps seed and named-file sizes
	// out of the engine's memory budget without imposing a specification limit.
	digest := func(name string) (string, error) {
		resolved, e := resolve(name)
		if e != nil {
			return "", e
		}
		f, e := openRegular(root, resolved, open)
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
	// Only schema positions naming computations are inspected. An arbitrary
	// literal may itself have keys such as argv or declared_files.
	for _, computation := range subject.Computations(s) {
		names := inputArray(computation["declared_files"])
		names = append(strictjson.Array{}, names...)
		argv := inputArray(computation["argv"])
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
				return in, safeReadFailure("QS8", "named file", e)
			}
			in.FileDigests[filepath.ToSlash(filepath.Clean(name))] = sum
		}
	}
	if seed, ok := s["seed"].(string); ok {
		resolved, e := resolve(seed)
		if e != nil {
			return in, safeReadFailure("QS9", "seed directory", e)
		}
		seed = filepath.ToSlash(resolved)
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
			if entry.IsDir() {
				f, e := root.Open(path)
				if e != nil {
					return e
				}
				_, e = f.ReadDir(-1)
				if e = errors.Join(e, f.Close()); e != nil {
					return e
				}
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
			if rel == ".pql-tmp" || strings.HasPrefix(filepath.ToSlash(rel), ".pql-tmp/") {
				return errSpecial
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
	return in, validateFileContents(s, in, root, resolve, open)
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
	return failure(2, rule, kind+" cannot be safely read: "+category)
}

func inside(directory, target string) bool {
	rel, err := filepath.Rel(directory, target)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

// validateFileContents gathers file evidence; subject owns the content rules.
func validateFileContents(s subject.File, in snapshot, root *os.Root, resolve func(string) (string, error), open func(string) (*os.File, error)) error {
	evidence := subject.FileEvidence{BinaryDigest: in.BinaryDigest}
	for _, digest := range in.FileDigests {
		evidence.NamedDigests = append(evidence.NamedDigests, digest)
	}
	seedFiles := map[string]bool{}
	for _, entry := range in.Seed {
		evidence.Seed = append(evidence.Seed, subject.FileEntry{Path: entry.Path, Directory: entry.Directory, Digest: entry.Digest})
		if !entry.Directory {
			seedFiles[entry.Path] = true
		}
	}
	if in.BinaryDigest != nil {
		for _, computation := range subject.Computations(s) {
			argv := inputArray(computation["argv"])
			if len(argv) == 0 {
				continue
			}
			name, ok := argv[0].(string)
			if !ok || !(strings.HasPrefix(filepath.Clean(name), "/bin/") || strings.HasPrefix(filepath.Clean(name), "/usr/bin/")) {
				continue
			}
			info, e := os.Stat(name)
			if e != nil || !info.Mode().IsRegular() {
				continue
			}
			f, e := os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			if e != nil {
				continue
			}
			h := sha256.New()
			_, e = io.Copy(h, f)
			e = errors.Join(e, f.Close())
			if e != nil {
				return e
			}
			evidence.NamedDigests = append(evidence.NamedDigests, fmt.Sprintf("%x", h.Sum(nil)))
		}
	}

	secret := inputObject(s["secret"])
	secretPath := stringValue(secret["path"])
	if seedFiles[secretPath] {
		seed := stringValue(s["seed"])
		resolved, e := resolve(filepath.Join(seed, secretPath))
		if e != nil {
			return safeReadFailure("QS9", "seed", e)
		}
		f, e := openRegular(root, resolved, open)
		if e != nil {
			return safeReadFailure("QS9", "seed", e)
		}
		token := stringValue(secret["token"])
		count, e := tokenOccurrences(f, []byte(token))
		e = errors.Join(e, f.Close())
		if e != nil {
			return safeReadFailure("QS9", "seed", e)
		}
		evidence.SecretOccurrences = count
	}
	if refusal := subject.CheckFiles(s, evidence); refusal != nil {
		return failure(2, refusal.Rule, "invalid subject file")
	}
	return nil
}

// tokenOccurrences counts every starting position, including overlaps (E-20),
// while retaining at most one read block and one token's worth of boundary.
func tokenOccurrences(r io.Reader, token []byte) (int, error) {
	if len(token) == 0 {
		return 0, nil
	}
	buffer := make([]byte, 32<<10)
	tail := []byte{}
	count := 0
	for {
		n, err := r.Read(buffer)
		data := append(tail, buffer[:n]...)
		for i := 0; i+len(token) <= len(data); i++ {
			if string(data[i:i+len(token)]) == string(token) {
				count++
			}
		}
		keep := len(token) - 1
		if keep > len(data) {
			keep = len(data)
		}
		tail = append([]byte(nil), data[len(data)-keep:]...)
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return 0, err
		}
	}
}

func resolveInput(directory, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", errEscape
	}
	target, e := filepath.EvalSymlinks(filepath.Join(directory, name))
	if e != nil {
		return "", e
	}
	if !inside(directory, target) {
		return "", errEscape
	}
	return filepath.Rel(directory, target)
}

func inputArray(v any) strictjson.Array {
	if a, ok := v.(strictjson.Array); ok {
		return a
	}
	return nil
}
func inputObject(v any) strictjson.Object {
	if o, ok := v.(strictjson.Object); ok {
		return o
	}
	return nil
}
