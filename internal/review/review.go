// Package review models explicit human review. It never infers reviewed or
// resolved state from file access, elapsed time, terminal output or Git writes.
package review

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"tavu/internal/files"
)

const SchemaVersion = 1
const NoHEAD = "UNBORN"
const MaxSnapshotBytes = 32 << 20

var ErrUnstable = errors.New("Files kept changing during capture; start the review again")

// Comparison contains original, unnormalized bytes. Modes/types distinguish
// absence, binary data and mode-only changes. HEAD and Baseline must identify
// the applicable comparison baseline; use NoHEAD for an unborn repository.
// Values are immutable by convention after construction; snapshots clone them.
type Comparison struct {
	Path     string `json:"path"`
	OldPath  string `json:"oldPath,omitempty"`
	Kind     string `json:"kind"`
	Old      []byte `json:"old"`
	New      []byte `json:"new"`
	OldMode  string `json:"oldMode,omitempty"`
	NewMode  string `json:"newMode,omitempty"`
	OldType  string `json:"oldType,omitempty"`
	NewType  string `json:"newType,omitempty"`
	HEAD     string `json:"head"`
	Baseline string `json:"baseline"`
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func (c Comparison) Fingerprint() string {
	identity := struct {
		Path, OldPath, Kind, OldHash, NewHash, OldMode, NewMode, OldType, NewType, HEAD, Baseline string
	}{c.Path, c.OldPath, c.Kind, digest(c.Old), digest(c.New), c.OldMode, c.NewMode, c.OldType, c.NewType, c.HEAD, c.Baseline}
	encoded, _ := json.Marshal(identity)
	return digest(encoded)
}

func (c Comparison) Clone() Comparison {
	c.Old = bytes.Clone(c.Old)
	c.New = bytes.Clone(c.New)
	return c
}

type Snapshot struct {
	ID          string       `json:"id"`
	Repository  string       `json:"repository"`
	Worktree    string       `json:"worktree"`
	HEAD        string       `json:"head"`
	CreatedAt   time.Time    `json:"createdAt"`
	Comparisons []Comparison `json:"comparisons"`
}

func newID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

// NewSnapshot copies only supplied comparisons. It is not an atomic repository
// snapshot. Use Capture for bounded concurrent-change detection during reads.
func NewSnapshot(repository, worktree, head string, comparisons []Comparison) (Snapshot, error) {
	if head == "" {
		head = NoHEAD
	}
	id, err := newID()
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{ID: id, Repository: repository, Worktree: worktree, HEAD: head, CreatedAt: time.Now().UTC(), Comparisons: make([]Comparison, len(comparisons))}
	total := 0
	for i, comparison := range comparisons {
		total += len(comparison.Old) + len(comparison.New)
		if total > MaxSnapshotBytes {
			return Snapshot{}, fmt.Errorf("Snapshot exceeds the %d MiB limit", MaxSnapshotBytes>>20)
		}
		if comparison.HEAD == "" {
			comparison.HEAD = head
		}
		snapshot.Comparisons[i] = comparison.Clone()
	}
	return snapshot, nil
}

func fingerprints(comparisons []Comparison, head string) string {
	items := make([]string, len(comparisons))
	for i, c := range comparisons {
		if c.HEAD == "" {
			c.HEAD = head
		}
		items[i] = c.Fingerprint()
	}
	sort.Strings(items)
	return strings.Join(items, "\n")
}

// Capture compares two complete reads and retries at most three times. The
// loader should put the observed HEAD/baseline in each comparison, and honor
// ctx. Sequential consistency checks detect common races, not all external
// process races, and do not make a multi-file read atomic.
func Capture(ctx context.Context, repository, worktree, head string, load func(context.Context) ([]Comparison, error)) (Snapshot, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		first, err := load(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		// Freeze the first read even if the loader reuses scratch buffers.
		firstSnapshot, err := NewSnapshot(repository, worktree, head, first)
		if err != nil {
			return Snapshot{}, err
		}
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		second, err := load(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		if fingerprints(firstSnapshot.Comparisons, firstSnapshot.HEAD) == fingerprints(second, firstSnapshot.HEAD) {
			return firstSnapshot, nil
		}
	}
	return Snapshot{}, ErrUnstable
}

type Side string

const (
	OldSide Side = "old"
	NewSide Side = "new"
)

type Comment struct {
	ID          string    `json:"id"`
	SnapshotID  string    `json:"snapshotId"`
	Path        string    `json:"path"`
	OldPath     string    `json:"oldPath,omitempty"`
	Kind        string    `json:"kind"`
	Fingerprint string    `json:"fingerprint"`
	Side        Side      `json:"side"`
	StartLine   int       `json:"startLine"`
	EndLine     int       `json:"endLine"`
	Code        []byte    `json:"code"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Resolved    bool      `json:"resolved"`
}

func (c Comment) BasedOnOldVersion(current Comparison) bool {
	return c.Fingerprint != current.Fingerprint()
}

type SeenRecord struct {
	Fingerprint string    `json:"fingerprint"`
	SeenAt      time.Time `json:"seenAt"`
}

// State is plain JSON data owned by the application. The caller serializes
// mutations; no UI objects, terminal content or environment values live here.
type State struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Seen          map[string]SeenRecord `json:"seen"`
	Comments      []Comment             `json:"comments"`
	Snapshots     []Snapshot            `json:"snapshots"`
}

func NewState() *State {
	return &State{SchemaVersion: SchemaVersion, Seen: make(map[string]SeenRecord), Comments: []Comment{}, Snapshots: []Snapshot{}}
}
func (s *State) MarkSeen(fingerprint string) {
	if s.Seen == nil {
		s.Seen = make(map[string]SeenRecord)
	}
	s.Seen[fingerprint] = SeenRecord{Fingerprint: fingerprint, SeenAt: time.Now().UTC()}
}
func (s *State) UnmarkSeen(fingerprint string)  { delete(s.Seen, fingerprint) }
func (s *State) IsSeen(fingerprint string) bool { _, ok := s.Seen[fingerprint]; return ok }

func (s *State) AddComment(snapshotID string, c Comparison, side Side, start, end int, body string) (Comment, error) {
	if strings.TrimSpace(body) == "" {
		return Comment{}, errors.New("Enter a review comment")
	}
	var raw []byte
	switch side {
	case OldSide:
		raw = c.Old
	case NewSide:
		raw = c.New
	default:
		return Comment{}, errors.New("Invalid comparison side")
	}
	if err := files.ValidateText(raw); err != nil {
		return Comment{}, err
	}
	code, err := files.Lines(raw, start, end)
	if err != nil {
		return Comment{}, err
	}
	id, err := newID()
	if err != nil {
		return Comment{}, err
	}
	now := time.Now().UTC()
	comment := Comment{ID: id, SnapshotID: snapshotID, Path: c.Path, OldPath: c.OldPath, Kind: c.Kind, Fingerprint: c.Fingerprint(), Side: side, StartLine: start, EndLine: end, Code: code, Body: body, CreatedAt: now, UpdatedAt: now}
	s.Comments = append(s.Comments, comment)
	// Return an independent copy so caller edits cannot alter stored code.
	comment.Code = bytes.Clone(comment.Code)
	return comment, nil
}

func (s *State) EditComment(id, body string) error {
	if strings.TrimSpace(body) == "" {
		return errors.New("Enter a review comment")
	}
	for i := range s.Comments {
		if s.Comments[i].ID == id {
			s.Comments[i].Body = body
			s.Comments[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.New("Comment not found")
}

func (s *State) SetResolved(id string, resolved bool) error {
	for i := range s.Comments {
		if s.Comments[i].ID == id {
			s.Comments[i].Resolved = resolved
			s.Comments[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.New("Comment not found")
}

func (s *State) DeleteComment(id string) error {
	for i := range s.Comments {
		if s.Comments[i].ID == id {
			copy(s.Comments[i:], s.Comments[i+1:])
			s.Comments[len(s.Comments)-1] = Comment{}
			s.Comments = s.Comments[:len(s.Comments)-1]
			return nil
		}
	}
	return errors.New("Comment not found")
}

func (s *State) Unresolved() []Comment {
	result := make([]Comment, 0)
	for _, comment := range s.Comments {
		if !comment.Resolved {
			comment.Code = bytes.Clone(comment.Code)
			result = append(result, comment)
		}
	}
	return result
}

// Retain limits snapshots, newest last; comments retain their original code
// even after their snapshot is removed. Return count enables explicit UI notice.
func (s *State) Retain(maxCount int, maxBytes int64) int {
	if maxCount < 0 {
		maxCount = 0
	}
	if maxBytes < 0 {
		maxBytes = 0
	}
	var used int64
	keepFrom := len(s.Snapshots)
	for i := len(s.Snapshots) - 1; i >= 0; i-- {
		var size int64
		for _, c := range s.Snapshots[i].Comparisons {
			size += int64(len(c.Old) + len(c.New))
		}
		if len(s.Snapshots)-i > maxCount || used+size > maxBytes {
			break
		}
		used += size
		keepFrom = i
	}
	if keepFrom > 0 {
		retained := append([]Snapshot(nil), s.Snapshots[keepFrom:]...)
		s.Snapshots = retained
	}
	return keepFrom
}

// Export formats exactly the supplied comments. Pass State.Unresolved() for
// the default selection, or a user-selected subset. It has no side effects.
func Export(workspaceLabel string, comments []Comment) string {
	if len(comments) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Workspace: %s\nManual review comments:\n", workspaceLabel)
	for i, c := range comments {
		side := "new"
		if c.Side == OldSide {
			side = "old"
		}
		fingerprint := c.Fingerprint
		if len(fingerprint) > 12 {
			fingerprint = fingerprint[:12]
		}
		fmt.Fprintf(&out, "\n%d. File: %s\n   Version and location: %s · %s · lines %d–%d · %s\n   Code:\n", i+1, c.Path, comparisonLabel(c.Kind), side, c.StartLine, c.EndLine, fingerprint)
		fence := strings.Repeat("`", max(3, longestBacktickRun(string(c.Code))+1))
		fmt.Fprintf(&out, "   %stext\n", fence)
		code := strings.TrimSuffix(strings.TrimSuffix(string(c.Code), "\n"), "\r")
		for _, line := range strings.Split(code, "\n") {
			fmt.Fprintf(&out, "   %s\n", strings.TrimSuffix(line, "\r"))
		}
		fmt.Fprintf(&out, "   %s\n   Comment:\n", fence)
		for _, line := range strings.Split(c.Body, "\n") {
			fmt.Fprintf(&out, "   %s\n", line)
		}
	}
	return out.String()
}

func comparisonLabel(kind string) string {
	switch kind {
	case "staged":
		return "Staged Changes"
	case "unstaged":
		return "Changes"
	case "untracked":
		return "Untracked"
	case "conflict":
		return "Merge Changes"
	default:
		return kind
	}
}

func longestBacktickRun(text string) int {
	longest, current := 0, 0
	for _, r := range text {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	return longest
}
