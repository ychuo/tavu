// Package terminal owns MyGo terminal instances. UI rebuilds only display them;
// they never start shells, duplicate terminal buffers, or replay commands.
package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
)

const ScrollbackBytes = 8 << 20

type Config struct {
	Dir        string
	Shell      string // executable path/name only; never a shell expression
	FontSize   float32
	FontFamily string
	Dispatch   func(func()) // schedules a callback on MyGo's UI thread
	OnChange   func()
	OnLink     func(LinkTarget)
	OnError    func(error)
}

// Session is a point-in-time status copy; Term remains the same instance until
// explicitly closed. Show it in at most one terminal.View per frame.
type Session struct {
	ID          string
	Label       string // stable configured-shell name; Title may change via OSC
	CustomLabel string // optional user name, independent of the process title
	Title       string
	Term        *mygoterm.Terminal
	Exited      bool
	ExitCode    int
	Error       string
}

type Manager struct {
	mu       sync.Mutex
	config   Config
	sessions []Session
	closed   bool
}

func NewManager(config Config) *Manager {
	if config.FontSize < 8 || config.FontSize > 32 {
		config.FontSize = 12
	}
	config.FontFamily = strings.TrimSpace(config.FontFamily)
	return &Manager{config: config}
}

func (m *Manager) notify() {
	if m.config.OnChange != nil && m.config.Dispatch != nil {
		m.config.Dispatch(m.config.OnChange)
	}
}

func (m *Manager) report(err error) {
	if m.config.OnError != nil && m.config.Dispatch != nil {
		m.config.Dispatch(func() { m.config.OnError(err) })
	}
}

func (m *Manager) NewSession() (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Session{}, errors.New("Terminal manager is closed")
	}
	dir, err := filepath.Abs(m.config.Dir)
	if err != nil {
		return Session{}, err
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return Session{}, fmt.Errorf("Cannot start a terminal in the project directory: %s", dir)
	}
	command, err := loginShellCommand(m.config.Shell)
	if err != nil {
		return Session{}, fmt.Errorf("Shell %q was not found: %w", m.config.Shell, err)
	}
	label := "Terminal"
	if shell := m.config.Shell; shell != "" {
		label = cleanTitle(filepath.Base(shell))
	}
	var idBytes [12]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return Session{}, err
	}
	id := hex.EncodeToString(idBytes[:])
	term, err := mygoterm.New(mygoterm.Options{
		Command: command, Dir: dir, Scrollback: ScrollbackBytes,
		Env:   []string{"TERM_PROGRAM=Tavu"},
		Font:  mygoterm.Font{Family: terminalFontFamily(m.config.FontFamily), Size: m.config.FontSize},
		Theme: forestTheme(), DarkTheme: forestTheme(),
		ClipboardRead: mygoterm.ClipboardDeny, NoClipboardWrite: true,
		OnExit: func(code int) {
			m.mu.Lock()
			for i := range m.sessions {
				if m.sessions[i].ID == id {
					m.sessions[i].Exited, m.sessions[i].ExitCode = true, code
					if code == -1 {
						m.sessions[i].Error = "Process failed to start; see the terminal for details"
					}
					break
				}
			}
			m.mu.Unlock()
			m.notify()
		},
		OnTitle: func(title string) {
			m.updateTitle(id, title)
		},
		OpenLink: func(raw string) bool {
			// true consumes every link, including invalid protocols: returning
			// false permits MyGo to hand URLs to the system's default opener.
			target, err := ParseLink(dir, raw)
			if err != nil {
				m.report(err)
			} else if m.config.OnLink != nil {
				m.config.OnLink(target) // MyGo invokes OpenLink on the UI thread.
			}
			return true
		},
	})
	if err != nil {
		return Session{}, fmt.Errorf("Cannot create terminal (check bundled libghostty-vt): %w", err)
	}
	session := Session{ID: id, Label: label, Title: fmt.Sprintf("Terminal %d", len(m.sessions)+1), Term: term}
	m.sessions = append(m.sessions, session)
	return session, nil
}

func cleanTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	runes := []rune(s)
	if len(runes) > 60 {
		s = string(runes[:60]) + "…"
	}
	if s == "" {
		return "Terminal"
	}
	return s
}

func (m *Manager) Sessions() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Session(nil), m.sessions...)
}

func (m *Manager) updateTitle(id, title string) {
	m.mu.Lock()
	for i := range m.sessions {
		if m.sessions[i].ID == id {
			m.sessions[i].Title = cleanTitle(title)
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

// RenameSession sets a user-facing name without sending input to the shell.
// An empty name restores the automatic label; OSC updates remain available
// for the full process-title tooltip and never overwrite the custom name.
func (m *Manager) RenameSession(id, label string) error {
	label = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, label))
	if label != "" {
		label = cleanTitle(label)
	}
	m.mu.Lock()
	for i := range m.sessions {
		if m.sessions[i].ID == id {
			m.sessions[i].CustomLabel = label
			m.mu.Unlock()
			m.notify()
			return nil
		}
	}
	m.mu.Unlock()
	return errors.New("Terminal session is no longer available")
}

// Close must be called only after the UI confirms closing an active session.
// MyGo hangs up only the PTY it owns, with a bounded process-group fallback.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	var term *mygoterm.Terminal
	for i, session := range m.sessions {
		if session.ID == id {
			term = session.Term
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	if term == nil {
		return nil
	}
	return term.Close()
}

func (m *Manager) CloseAll() error {
	m.mu.Lock()
	m.closed = true
	sessions := m.sessions
	m.sessions = nil
	m.mu.Unlock()
	var errs []error
	for _, session := range sessions {
		if err := session.Term.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) SetFontSize(size float32) {
	if size < 8 || size > 32 {
		return
	}
	m.mu.Lock()
	m.config.FontSize = size
	family := terminalFontFamily(m.config.FontFamily)
	for _, session := range m.sessions {
		session.Term.SetFont(mygoterm.Font{Family: family, Size: size})
	}
	m.mu.Unlock()
}

// SetFontFamily applies to existing and future terminals without restarting
// their processes. Empty uses MyGo's system monospace font. Installed Nerd Font
// symbols are a fallback without changing the user's saved family preference.
func (m *Manager) SetFontFamily(family string) {
	m.mu.Lock()
	m.config.FontFamily = strings.TrimSpace(family)
	resolved := terminalFontFamily(m.config.FontFamily)
	for _, session := range m.sessions {
		session.Term.SetFont(mygoterm.Font{Family: resolved, Size: m.config.FontSize})
	}
	m.mu.Unlock()
}

// SetShell changes only future sessions. NewSession validates the executable;
// existing shells are never restarted and no commands are replayed.
func (m *Manager) SetShell(shell string) {
	m.mu.Lock()
	m.config.Shell = shell
	m.mu.Unlock()
}
