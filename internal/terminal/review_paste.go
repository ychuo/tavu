package terminal

import (
	"errors"
	"strings"
	"unicode"
)

var ErrPasteUnavailable = errors.New("Terminal session is no longer available")
var ErrPasteNeedsDraft = errors.New("This terminal has not enabled safe multiline paste. Your comments remain in the draft.")

// PasteReview never types a return or clears input. A terminal that has not
// explicitly requested bracketed paste receives no bytes at all. Callers keep
// the text as an application-owned draft when ErrPasteNeedsDraft is returned.
// Snapshot is MyGo's public terminal-state export, including DEC modes; Send
// receives an explicitly bracketed payload so a mode change cannot make
// MyGo.Paste silently fall back to newline-as-Return encoding.
func (m *Manager) PasteReview(id, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrPasteUnavailable
	}
	for _, s := range m.sessions {
		if s.ID == id && !s.Exited && s.Term != nil {
			return pasteReview(s.Term, text)
		}
	}
	return ErrPasteUnavailable
}

type reviewPasteTerminal interface {
	Snapshot() []byte
	Send([]byte)
}

func pasteReview(term reviewPasteTerminal, text string) error {
	if text == "" {
		return nil
	}
	if !bracketedPasteEnabled(term.Snapshot()) {
		return ErrPasteNeedsDraft
	}
	// Strip control bytes that could terminate the bracket or act as editing
	// commands. Preserve Unicode, tabs and LF inside the bracket, with no
	// Return after it and no command that replaces existing shell input.
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
	term.Send([]byte("\x1b[200~" + text + "\x1b[201~"))
	return nil
}

// The snapshot is state, not a screen-text guess. Unknown/absent modes fail
// closed. Scan CSI mode sets/resets, ignoring OSC/DCS strings completely.
func bracketedPasteEnabled(snapshot []byte) bool {
	enabled := false
	for i := 0; i < len(snapshot); i++ {
		if snapshot[i] != 0x1b || i+1 >= len(snapshot) {
			continue
		}
		i++
		switch snapshot[i] {
		case 'c':
			enabled = false
		case ']', 'P', '^', '_':
			for i++; i < len(snapshot); i++ {
				if snapshot[i] == 7 || (snapshot[i] == 0x1b && i+1 < len(snapshot) && snapshot[i+1] == '\\') {
					break
				}
			}
		case '[':
			start := i + 1
			for i++; i < len(snapshot); i++ {
				if snapshot[i] >= 0x40 && snapshot[i] <= 0x7e {
					if (snapshot[i] == 'h' || snapshot[i] == 'l') && start < i && snapshot[start] == '?' {
						for _, mode := range strings.Split(string(snapshot[start+1:i]), ";") {
							if mode == "2004" {
								enabled = snapshot[i] == 'h'
							}
						}
					}
					break
				}
			}
		}
	}
	return enabled
}
