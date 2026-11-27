// Package store persists decided records separately from disposable workspaces.
// The command resolves the application-support directory after root refusal;
// this package never resolves a default location during construction.
package store

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

var ErrRootRequired = errors.New("an absolute store root is required")

// ErrNotStored tells the exit funnel to use exit 70 and print this scorecard note.
var ErrNotStored = errors.New("record was not stored")

type Store struct{ root string }

// New requires the caller's explicit durable root. Workspace paths belong only
// to throwaway; they are never placed below this directory.
func New(root string) (*Store, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, ErrRootRequired
	}
	if err := privateDirectory(root); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}
func privateDirectory(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	if canonical != filepath.Join(parent, filepath.Base(path)) {
		return errors.New("store directory must not contain links")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("store directory must have mode 0700")
	}
	return nil
}

// ApplicationSupportRoot is used only by the command, after its root refusal.
// On darwin UserConfigDir resolves the user's application-support directory.
func ApplicationSupportRoot() (string, error) {
	parent, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, "pql"), nil
}

// Record contains the stored envelope, not evidence collection policy. Judge
// values are the already masked values used by the single grade encoding path.
// Host must contain the actual host facts supplied by the caller (KTD8).
// SubjectFilePath must be the canonical absolute subject-file path.
type Record struct {
	Host            strictjson.Object
	SubjectFilePath string
	// DisplaySubjectFilePath is masked by grade; the canonical path still keys storage.
	DisplaySubjectFilePath *string
	SubjectFileDigest      string
	BinaryDigest           *string
	SeedDigest             *string
	Request                string
	Scope                  strictjson.Array
	ExitCode               int
	JudgeInput             any
	JudgeOutput            any
	JudgeRejected          bool
}

func nullable(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// Document builds the envelope without validating judge input. QE15 and errata
// E-5 require even a schema-invalid input to remain available for diagnosis.
func (r Record) Document() strictjson.Object {
	path := r.SubjectFilePath
	if r.DisplaySubjectFilePath != nil {
		path = *r.DisplaySubjectFilePath
	}
	return strictjson.Object{
		"record_version":       "1.0",
		"spec_version":         "1.0",
		"host":                 r.Host,
		"subject_file_path":    path,
		"subject_file_digest":  r.SubjectFileDigest,
		"binary_digest":        nullable(r.BinaryDigest),
		"seed_digest":          nullable(r.SeedDigest),
		"request":              r.Request,
		"scope":                r.Scope,
		"exit_code":            r.ExitCode,
		"judge_input":          r.JudgeInput,
		"judge_output":         r.JudgeOutput,
		"judge_rejected":       r.JudgeRejected,
		"next_actions":         strictjson.Array{},
		"offline_settings":     strictjson.Array{},
		"bookkeeping_excluded": strictjson.Array{},
	}
}

// Write serializes with the contract writer and publishes by rename. The path
// key uses the canonical subject path; the record key uses its complete bytes.
// Any failure, including a close or rename failure, is returned to the funnel.
func (s *Store) Write(r Record) (path string, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(ErrNotStored, err)
		}
	}()
	if s == nil || s.root == "" {
		return "", ErrRootRequired
	}
	if !filepath.IsAbs(r.SubjectFilePath) || filepath.Clean(r.SubjectFilePath) != r.SubjectFilePath {
		return "", errors.New("canonical subject path required")
	}
	if err = privateDirectory(s.root); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(r.SubjectFilePath)))
	dir := filepath.Join(s.root, key)
	if err = privateDirectory(dir); err != nil {
		return "", err
	}
	data := strictjson.Write(r.Document())
	f, err := os.CreateTemp(dir, ".record-")
	if err != nil {
		return "", err
	}
	// A failed write can leave a private temporary file for diagnosis. It is
	// never named as a record or consulted for a verdict.
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return "", err
	}
	path = filepath.Join(dir, fmt.Sprintf("%x.json", sha256.Sum256(data)))
	if err = os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// Workspace creates and atomically publishes a run through its sole owner.
// The caller supplies the engine's process identity and the process lookup.
func Workspace(parent string, process throwaway.Process) (*throwaway.Workspace, error) {
	w, err := throwaway.Create(parent)
	if err != nil {
		return nil, err
	}
	if err = w.Publish(process); err != nil {
		return nil, errors.Join(err, w.Remove())
	}
	return w, nil
}

// Sweep never inspects a workspace entry itself.
func Sweep(parent string, lookup throwaway.ProcessLookup) error {
	return throwaway.Sweep(parent, lookup)
}
