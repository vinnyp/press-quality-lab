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
	"runtime"
	"strconv"
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

type inputAccess struct {
	canonical       string
	subjectInfo     os.FileInfo
	stat            func(string) (os.FileInfo, error)
	systemPath      func(string) bool
	openAbsolute    func(string) (*os.File, error)
	resolveAbsolute func(string) (string, error)
	statAbsolute    func(string) (os.FileInfo, error)
	copyAbsolute    func(io.Writer, io.Reader) (int64, error)
	home            func() (string, error)
	open            func(*os.Root, string) (*os.File, error)
}

func accountHome() (string, error) {
	// LookupId fails closed instead of accepting os/user's pure-Go $HOME fallback.
	var account *user.User
	var err error
	if runtime.GOOS == "darwin" {
		account, err = user.Current()
	} else {
		account, err = user.LookupId(strconv.Itoa(os.Getuid()))
	}
	if err != nil {
		return "", err
	}
	return account.HomeDir, nil
}

// planningInput snapshots binary, seed and computation files without execution.
func planningInput(o Options, s subject.File) (snapshot, error) {
	return planningInputWith(o, s, inputAccess{})
}

func planningInputWith(o Options, s subject.File, access inputAccess) (in snapshot, err error) {
	in.Input = plan.Input{Subject: s, Baseline: o.Baseline != "", Anchor: o.Anchor, FileDigests: map[string]string{}}
	for _, token := range o.Scope {
		in.Scopes = append(in.Scopes, rulebook.Scope(token))
	}
	canonical := access.canonical
	var e error
	if canonical == "" {
		canonical, e = physicalSubjectPath(o.SubjectFile)
	}
	if e != nil {
		return in, safeReadFailure("QS8", "subject directory", e)
	}
	directory := filepath.Dir(canonical)
	in.SubjectDirectory = directory
	root, err := os.OpenRoot(directory)
	if err != nil {
		return in, safeReadFailure("QS8", "subject directory", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if access.subjectInfo != nil {
		info, e := root.Lstat(filepath.Base(canonical))
		if e != nil {
			return in, safeReadFailure("QS8", "subject directory", e)
		}
		if !os.SameFile(info, access.subjectInfo) {
			return in, safeReadFailure("QS8", "subject directory", errEscape)
		}
	}
	if access.stat == nil {
		access.stat = os.Stat
	}

	if access.home == nil {
		access.home = accountHome
	}
	home, e := access.home()
	if e != nil {
		return in, e
	}
	home, e = existingHomePrefix(home)
	if e != nil {
		return in, e
	}
	directoryInfo, e := root.Stat(".")
	if e != nil {
		return in, safeReadFailure("QS8", "subject directory", e)
	}
	for ancestor := home; ; ancestor = filepath.Dir(ancestor) {
		info, e := access.stat(ancestor)
		if missingPath(e) && filepath.Dir(ancestor) != ancestor {
			continue
		}
		if e != nil {
			return in, e
		}
		if os.SameFile(directoryInfo, info) {
			return in, safeReadFailure("QS8", "subject directory", errHome)
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	resolve := func(name string) (string, error) { return resolveInput(directory, name) }
	open := func(name string) (*os.File, error) {
		if access.open != nil {
			return access.open(root, name)
		}
		return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	}

	// Only digests enter the plan; streaming keeps seed and named-file sizes
	// out of the engine's memory budget without imposing a specification limit.
	memo := map[string]string{}
	digest := func(name string) (string, error) {
		resolved, e := resolve(name)
		if e != nil {
			return "", e
		}
		if sum, ok := memo[filepath.Join(directory, resolved)]; ok {
			return sum, nil
		}
		f, e := openRegular(root, resolved, open)
		if e != nil {
			return "", e
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		e = errors.Join(e, f.Close())
		sum := fmt.Sprintf("%x", h.Sum(nil))
		if e == nil {
			memo[filepath.Join(directory, resolved)] = sum
		}
		return sum, e
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
	return in, validateFileContents(s, in, root, resolve, open, access, memo)
}

func openInput(path string) (io.ReadCloser, error) {
	return openInputWith(path, func(name string) (*os.File, error) { return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) })
}

func openInputWith(path string, open func(string) (*os.File, error)) (io.ReadCloser, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errSpecial
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(err, errSpecial, f.Close())
	}
	return f, nil
}

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
	errHome    = errors.New("home")
	errEscape  = errors.New("escape")
	errLink    = errors.New("link")
	errSpecial = errors.New("special")
)

// safeReadFailure exposes only a category, never an underlying path or value.
func safeReadFailure(rule, kind string, err error) *Error {
	category := "special"
	switch {
	case errors.Is(err, os.ErrPermission):
		category = "unreadable"
	case errors.Is(err, errHome):
		category = "home"
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
func validateFileContents(s subject.File, in snapshot, root *os.Root, resolve func(string) (string, error), open func(string) (*os.File, error), access inputAccess, digests map[string]string) error {
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
	if access.systemPath == nil {
		access.systemPath = func(name string) bool {
			return strings.HasPrefix(filepath.Clean(name), "/bin/") || strings.HasPrefix(filepath.Clean(name), "/usr/bin/")
		}
	}
	if access.openAbsolute == nil {
		access.openAbsolute = func(name string) (*os.File, error) { return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) }
	}
	if access.resolveAbsolute == nil {
		access.resolveAbsolute = filepath.EvalSymlinks
	}
	if access.statAbsolute == nil {
		access.statAbsolute = os.Stat
	}
	if access.copyAbsolute == nil {
		access.copyAbsolute = io.Copy
	}
	seen := map[string]bool{}
	if in.BinaryDigest != nil {
		for _, computation := range subject.Computations(s) {
			argv := inputArray(computation["argv"])
			if len(argv) == 0 {
				continue
			}
			name, ok := argv[0].(string)
			if !ok || !access.systemPath(name) {
				continue
			}
			resolved, e := access.resolveAbsolute(name)
			if missingPath(e) {
				continue
			}
			if e != nil {
				return safeReadFailure("QS12", "absolute executable", e)
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			if sum, exists := digests[resolved]; exists {
				evidence.NamedDigests = append(evidence.NamedDigests, sum)
				continue
			}
			info, e := access.statAbsolute(resolved)
			if missingPath(e) {
				continue
			}
			if e != nil {
				return safeReadFailure("QS12", "absolute executable", e)
			}
			if !info.Mode().IsRegular() {
				return safeReadFailure("QS12", "absolute executable", errSpecial)
			}
			f, e := access.openAbsolute(resolved)
			if missingPath(e) {
				continue
			}
			if e != nil {
				return safeReadFailure("QS12", "absolute executable", e)
			}
			info, e = f.Stat()
			if e != nil || !info.Mode().IsRegular() {
				return safeReadFailure("QS12", "absolute executable", errors.Join(e, errSpecial, f.Close()))
			}

			h := sha256.New()
			_, e = access.copyAbsolute(h, f)
			e = errors.Join(e, f.Close())
			if e != nil {
				return safeReadFailure("QS12", "absolute executable", e)
			}
			sum := fmt.Sprintf("%x", h.Sum(nil))
			digests[resolved] = sum
			evidence.NamedDigests = append(evidence.NamedDigests, sum)
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

// physicalSubjectPath preserves link/.. until the filesystem resolves it.
func physicalSubjectPath(name string) (string, error) {
	if !filepath.IsAbs(name) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		cwd, err = filepath.EvalSymlinks(cwd)
		if err != nil {
			return "", err
		}
		name = cwd + string(os.PathSeparator) + name
	}
	return filepath.EvalSymlinks(name)
}

func missingPath(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func existingHomePrefix(home string) (string, error) {
	if !filepath.IsAbs(home) {
		return "", errors.New("account home is not absolute")
	}
	// Peel missing suffixes without cleaning link/.. lexically. Readlink also
	// follows a dangling final link, whose target may have different ancestors.
	links := 0
	for {
		resolved, err := filepath.EvalSymlinks(home)
		if err == nil {
			return resolved, nil
		}
		if !missingPath(err) {
			return "", err
		}
		target, linkErr := os.Readlink(home)
		if linkErr == nil {
			links++
			if links > 255 {
				return "", syscall.ELOOP
			}
			if filepath.IsAbs(target) {
				home = target
			} else {
				home = home[:strings.LastIndexByte(home, '/')] + "/" + target
			}
			continue
		}
		if !missingPath(linkErr) && !errors.Is(linkErr, syscall.EINVAL) {
			return "", linkErr
		}
		home = strings.TrimRight(home, "/")
		last := strings.LastIndexByte(home, '/')
		if last <= 0 {
			home = "/"
		} else {
			home = home[:last]
		}
	}
}
