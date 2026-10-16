// Package gitrepo provides bounded, cancellable access to the user's Git binary.
// It never invokes a shell and never treats a path as a Git pathspec expression.
package gitrepo

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	Staged    Kind = "staged"
	Unstaged  Kind = "unstaged"
	Untracked Kind = "untracked"
	Conflict  Kind = "conflict"
)

var (
	ErrNotRepository = errors.New("This folder is not in a Git worktree")
	ErrStale         = errors.New("Git status changed; refresh and try again")
	ErrUnstable      = errors.New("Files changed during capture; try again")
	ErrConflict      = errors.New("Merge conflicts found; resolve them externally and refresh")
	ErrTooLarge      = errors.New("File exceeds the preview size limit")
	ErrOutputLimit   = errors.New("Git output exceeds the size limit")
	ErrNoChanges     = errors.New("No staged changes to commit")
)

type Options struct {
	QueryTimeout   time.Duration
	WriteTimeout   time.Duration
	MaxOutputBytes int64
	MaxFileBytes   int64
}

func (o Options) defaults() Options {
	if o.QueryTimeout <= 0 {
		o.QueryTimeout = 10 * time.Second
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = 2 * time.Minute
	}
	if o.MaxOutputBytes <= 0 {
		o.MaxOutputBytes = 16 << 20
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = 4 << 20
	}
	return o
}

type Repository struct {
	Root      string
	GitDir    string
	CommonDir string
	options   Options
	writes    sync.Mutex
}

type Change struct {
	ID          string `json:"id"`
	Kind        Kind   `json:"kind"`
	Path        string `json:"path"`
	OldPath     string `json:"oldPath,omitempty"`
	Code        string `json:"code"`
	StatusToken string `json:"-"`
}

type Status struct {
	Root      string
	GitDir    string
	CommonDir string
	Branch    string
	HEAD      string // Empty only when Unborn is true.
	Unborn    bool
	Detached  bool
	Token     string
	Changes   []Change
}

// Version keeps original bytes separate from any editor's normalized buffer.
// A missing side has Exists=false; an empty file has Exists=true.
type Version struct {
	Content []byte `json:"content"`
	Mode    string `json:"mode"`
	OID     string `json:"oid,omitempty"`
	Exists  bool   `json:"exists"`
}

type Comparison struct {
	Change      Change
	Old         Version
	New         Version
	HEAD        string
	Branch      string
	Baseline    string
	Binary      bool
	Unsupported string
	CapturedAt  time.Time
}

type CommandError struct {
	Args   []string
	Stderr string
	Stdout string
	Cause  error
}

func (e *CommandError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(e.Stdout)
	}
	if detail == "" {
		detail = e.Cause.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), detail)
}
func (e *CommandError) Unwrap() error { return e.Cause }
