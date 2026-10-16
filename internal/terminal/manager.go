// Package terminal owns MyGo terminal instances. UI rebuilds only display them;
// they never start shells, duplicate terminal buffers, or replay commands.
package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
)

const ScrollbackBytes = 8 << 20

type Config struct {
	Dir      string
	Shell    string // executable path/name only; never a shell expression
	FontSize float32
	Dispatch func(func()) // schedules a callback on MyGo's UI thread
	OnChange func()
	OnLink   func(LinkTarget)
	OnError  func(error)
}

// Session is a point-in-time status copy; Term remains the same instance until
// explicitly closed. Show it in at most one terminal.View per frame.
type Session struct {
	ID       string
	Label    string // stable configured-shell name; Title may change via OSC
	Title    string
	Term     *mygoterm.Terminal
	Exited   bool
	ExitCode int
	Error    string
}

type Manager struct {
	mu       sync.Mutex
	config   Config
	sessions []Session
	closed   bool
}

func NewManager(config Config) *Manager {
	if config.FontSize < 8 || config.FontSize > 32 {
		config.FontSize = 14
	}
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
	var command []string
	label := "Terminal"
	if shell := m.config.Shell; shell != "" {
		path, err := exec.LookPath(shell)
		if err != nil {
			return Session{}, fmt.Errorf("Configured shell %q was not found: %w", shell, err)
		}
		command = []string{path, "-l"}
		label = cleanTitle(filepath.Base(shell))
	}
	var idBytes [12]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return Session{}, err
	}
	id := hex.EncodeToString(idBytes[:])
	term, err := mygoterm.New(mygoterm.Options{
		Command: command, Dir: dir, Scrollback: ScrollbackBytes,
		Font:  mygoterm.Font{Size: m.config.FontSize},
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
			m.mu.Lock()
			for i := range m.sessions {
				if m.sessions[i].ID == id {
					m.sessions[i].Title = cleanTitle(title)
					break
				}
			}
			m.mu.Unlock()
			m.notify()
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
	for _, session := range m.sessions {
		session.Term.SetFont(mygoterm.Font{Size: size})
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
