// Package storage stores only settings and explicitly captured review data.
// Callers provide mygo.App.Path(mygo.PathUserData); this package never writes
// into a workspace, Git metadata, or an old product's data directory.
package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	SchemaVersion                 = 1
	DefaultMaxSnapshotBytes int64 = 64 << 20
	DefaultMaxSnapshots           = 20
	maxMetadataBytes        int64 = 16 << 20
)

type Layout struct {
	Mode               string  `json:"mode"`
	SplitTerminals     bool    `json:"splitTerminals"`
	SidebarVisible     bool    `json:"sidebarVisible"`
	SidebarWidth       float32 `json:"sidebarWidth"`
	TerminalRatio      float32 `json:"terminalRatio"`
	StackedReaderRatio float32 `json:"stackedReaderRatio,omitempty"`
	Arrangement        string  `json:"arrangement,omitempty"`
}

type Settings struct {
	Theme            string   `json:"theme"`
	Language         string   `json:"language"`
	RecentProjects   []string `json:"recentProjects"`
	PinnedProjects   []string `json:"pinnedProjects,omitempty"`
	LastProject      string   `json:"lastProject"`
	Shell            string   `json:"shell"`
	FontSize         float32  `json:"fontSize"`
	FontFamily       string   `json:"fontFamily"`
	Layout           Layout   `json:"layout"`
	MaxSnapshotBytes int64    `json:"maxSnapshotBytes"`
	MaxSnapshots     int      `json:"maxSnapshots"`
}

func DefaultSettings() Settings {
	return Settings{Theme: "dark", Language: "en", FontSize: 12, Layout: Layout{Mode: "split", SidebarVisible: true, SidebarWidth: 280, TerminalRatio: .48, StackedReaderRatio: .66, Arrangement: "auto"}, MaxSnapshotBytes: DefaultMaxSnapshotBytes, MaxSnapshots: DefaultMaxSnapshots}
}

func (s *Settings) normalize() {
	if s.Theme != "light" && s.Theme != "system" {
		s.Theme = "dark"
	}
	switch s.Language {
	case "en", "zh-Hans", "zh-Hant", "ja":
	default:
		s.Language = "en"
	}
	if s.FontSize < 8 || s.FontSize > 32 {
		s.FontSize = 12
	}
	s.FontFamily = strings.TrimSpace(s.FontFamily)
	if s.Layout.Mode != "split" && s.Layout.Mode != "terminal" && s.Layout.Mode != "review" {
		s.Layout.Mode = "split"
	}
	if s.Layout.TerminalRatio < .15 || s.Layout.TerminalRatio > .85 {
		s.Layout.TerminalRatio = .48
	}
	if s.Layout.StackedReaderRatio < .25 || s.Layout.StackedReaderRatio > .85 {
		s.Layout.StackedReaderRatio = .66
	}
	if s.Layout.Arrangement != "horizontal" && s.Layout.Arrangement != "vertical" {
		s.Layout.Arrangement = "auto"
	}
	if s.Layout.SidebarWidth < 200 || s.Layout.SidebarWidth > 480 {
		s.Layout.SidebarWidth = 280
	}
	if s.MaxSnapshotBytes < 1<<20 || s.MaxSnapshotBytes > 1<<30 {
		s.MaxSnapshotBytes = DefaultMaxSnapshotBytes
	}
	if s.MaxSnapshots < 1 || s.MaxSnapshots > 100 {
		s.MaxSnapshots = DefaultMaxSnapshots
	}
	s.RecentProjects = normalizedProjectPaths(s.RecentProjects, 10)
	// Pins have their own ordering and remain available after a folder ages out
	// of the bounded recent history. Older settings simply have no pins.
	s.PinnedProjects = normalizedProjectPaths(s.PinnedProjects, 0)
}

func normalizedProjectPaths(paths []string, limit int) []string {
	seen := make(map[string]bool)
	var result []string
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
		if limit > 0 && len(result) == limit {
			break
		}
	}
	return result
}

type envelope struct {
	SchemaVersion int             `json:"schemaVersion"`
	Data          json.RawMessage `json:"data"`
}

type CorruptError struct {
	Path, BackupPath string
	Cause            error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("Local data is corrupt; original preserved. Backup: %s (%v)", e.BackupPath, e.Cause)
}
func (e *CorruptError) Unwrap() error { return e.Cause }

type Store struct {
	mu           sync.Mutex
	root         string
	maxBytes     int64
	maxSnapshots int
	blocked      map[string]error
}

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("App data directory is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, maxBytes: DefaultMaxSnapshotBytes, maxSnapshots: DefaultMaxSnapshots, blocked: make(map[string]error)}, nil
}

func (s *Store) Root() string { return s.root }

// SetLimits applies to subsequent writes. Existing history is not removed until
// the next explicit snapshot or ClearSnapshots call.
func (s *Store) SetLimits(maxBytes int64, maxSnapshots int) {
	settings := DefaultSettings()
	settings.MaxSnapshotBytes = maxBytes
	settings.MaxSnapshots = maxSnapshots
	settings.normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxBytes = settings.MaxSnapshotBytes
	s.maxSnapshots = settings.MaxSnapshots
}

func (s *Store) LoadSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := DefaultSettings()
	err := s.read(filepath.Join(s.root, "settings.json"), &settings, maxMetadataBytes)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return DefaultSettings(), err
	}
	settings.normalize()
	return settings, nil
}

func (s *Store) SaveSettings(settings Settings) error {
	settings.normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(filepath.Join(s.root, "settings.json"), settings, maxMetadataBytes)
}

func (s *Store) ProjectDir(project string) string {
	// Worktrees are deliberately separate: absolute workspace identity, not the
	// shared Git common directory, keys the storage directory.
	if p, err := filepath.Abs(project); err == nil {
		project = filepath.Clean(p)
	}
	hash := sha256.Sum256([]byte(project))
	return filepath.Join(s.root, "projects", hex.EncodeToString(hash[:]))
}

func (s *Store) LoadProject(project string, out any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(filepath.Join(s.ProjectDir(project), "review.json"), out, maxMetadataBytes)
}

func (s *Store) SaveProject(project string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(filepath.Join(s.ProjectDir(project), "review.json"), value, maxMetadataBytes)
}

type SnapshotInfo struct {
	ID         string
	Size       int64
	ModifiedAt time.Time
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (s *Store) SaveSnapshot(project, id string, value any) error {
	if !validID(id) {
		return errors.New("Invalid snapshot ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.ProjectDir(project), "snapshots")
	if _, err := os.Lstat(filepath.Join(dir, id+".json")); err == nil {
		return errors.New("Snapshot ID already exists; historical content cannot be overwritten")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.write(filepath.Join(dir, id+".json"), value, s.maxBytes); err != nil {
		return err
	}
	items, err := s.listSnapshots(project)
	if err != nil {
		return err
	}
	var total int64
	for _, item := range items {
		total += item.Size
	}
	count := len(items)
	for _, item := range items {
		if count <= s.maxSnapshots && total <= s.maxBytes {
			break
		}
		if item.ID == id {
			continue
		}
		if err := os.Remove(filepath.Join(dir, item.ID+".json")); err != nil {
			return err
		}
		count--
		total -= item.Size
	}
	return nil
}

func (s *Store) LoadSnapshot(project, id string, out any) error {
	if !validID(id) {
		return errors.New("Invalid snapshot ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(filepath.Join(s.ProjectDir(project), "snapshots", id+".json"), out, s.maxBytes)
}

func (s *Store) ListSnapshots(project string) ([]SnapshotInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listSnapshots(project)
}

func (s *Store) listSnapshots(project string) ([]SnapshotInfo, error) {
	entries, err := os.ReadDir(filepath.Join(s.ProjectDir(project), "snapshots"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []SnapshotInfo
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !validID(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		result = append(result, SnapshotInfo{ID: id, Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ModifiedAt.Equal(result[j].ModifiedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].ModifiedAt.Before(result[j].ModifiedAt)
	})
	return result, nil
}

func (s *Store) ClearSnapshots(project string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(filepath.Join(s.ProjectDir(project), "snapshots"))
}

func (s *Store) read(path string, out any, limit int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("Local data exceeds the read limit (%d bytes): %s", limit, path)
	}
	var env envelope
	if err = json.Unmarshal(data, &env); err != nil {
		return s.corrupt(path, data, err)
	}
	if env.SchemaVersion != SchemaVersion {
		return fmt.Errorf("Unsupported local data schemaVersion %d: %s; original preserved", env.SchemaVersion, path)
	}
	if len(env.Data) == 0 || bytes.Equal(env.Data, []byte("null")) {
		return s.corrupt(path, data, errors.New("Missing data field"))
	}
	if err = json.Unmarshal(env.Data, out); err != nil {
		return s.corrupt(path, data, err)
	}
	return nil
}

func (s *Store) write(path string, value any, limit int64) error {
	// A failed load must never lead to silently overwriting corrupt/foreign data.
	if err := s.blocked[path]; err != nil {
		return err
	}
	var old json.RawMessage
	if err := s.read(path, &old, limit); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if bytes.Equal(payload, []byte("null")) {
		return errors.New("Cannot save empty local data")
	}
	data, err := json.MarshalIndent(envelope{SchemaVersion: SchemaVersion, Data: payload}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if int64(len(data)) > limit {
		return fmt.Errorf("Local data exceeds the save limit (%d bytes)", limit)
	}
	return atomicWrite(path, data)
}

func (s *Store) corrupt(path string, data []byte, cause error) error {
	err := preserveCorrupt(path, data, cause)
	s.blocked[path] = err
	return err
}

func preserveCorrupt(path string, data []byte, cause error) error {
	hash := sha256.Sum256(data)
	backup := path + ".corrupt-" + hex.EncodeToString(hash[:6])
	f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		_, err = f.Write(data)
		if err == nil {
			err = f.Sync()
		}
		err = errors.Join(err, f.Close())
	} else if errors.Is(err, os.ErrExist) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("Local data is corrupt and backup failed; original preserved: %s: %w", path, err)
	}
	return &CorruptError{Path: path, BackupPath: backup, Cause: cause}
}

// RecoverCorrupt is an explicit user-initiated recovery operation. It only
// removes a corrupt original once an identical backup exists. A later write
// may then create fresh data; neither the backup nor old product data is erased.
func (s *Store) RecoverCorrupt(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("Only files in the Tavu data directory can be recovered")
	}
	var out json.RawMessage
	err = s.read(abs, &out, s.maxBytes)
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		return errors.New("This file is not a recoverable corrupt record")
	}
	if err := os.Remove(abs); err != nil {
		return err
	}
	delete(s.blocked, abs)
	return nil
}

func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tavu-write-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer func() { f.Close(); os.Remove(temp) }()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp, path); err != nil {
		return err
	}
	if d, openErr := os.Open(dir); openErr == nil {
		err = d.Sync()
		err = errors.Join(err, d.Close())
	}
	return err
}
