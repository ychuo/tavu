package terminal

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
)

type pasteRecorder struct {
	state []byte
	input []byte
}

func (r *pasteRecorder) Snapshot() []byte { return r.state }
func (r *pasteRecorder) Send(b []byte)    { r.input = append(r.input, b...) }

func TestReviewPasteRequiresExplicitBracketedMode(t *testing.T) {
	for _, state := range []string{"", "screen says ?2004h", "\x1b[?2004l", "\x1b[?2004h\x1b[?2004l", "\x1b[?2004h\x1bc", "\x1b]2;\x1b[?2004h\a"} {
		r := &pasteRecorder{state: []byte(state), input: []byte("existing input")}
		if err := pasteReview(r, "first\nsecond\n"); !errors.Is(err, ErrPasteNeedsDraft) {
			t.Fatalf("state %q: expected draft fallback, got %v", state, err)
		}
		if string(r.input) != "existing input" {
			t.Fatalf("unsupported state sent bytes: %q", r.input)
		}
	}
}

func TestReviewPastePreservesInputAndNeverAddsReturn(t *testing.T) {
	r := &pasteRecorder{state: []byte("\x1b[?1;2004h"), input: []byte("existing input ")}
	if err := pasteReview(r, "中文\nsecond\n"); err != nil {
		t.Fatal(err)
	}
	want := "existing input \x1b[200~中文\nsecond\n\x1b[201~"
	if string(r.input) != want {
		t.Fatalf("paste replaced input, added Enter, or lost framing:\ngot %q\nwant %q", r.input, want)
	}
	r.input = nil
	if err := pasteReview(r, "safe\x1b[201~\x15\r\nnext\x03"); err != nil {
		t.Fatal(err)
	}
	if string(r.input) != "\x1b[200~safe[201~\nnext\x1b[201~" {
		t.Fatalf("control characters escaped paste framing: %q", r.input)
	}
}

func TestReviewPasteRejectsClosedOrMissingSession(t *testing.T) {
	m := NewManager(Config{})
	m.sessions = []Session{{ID: "exited", Exited: true}}
	for _, id := range []string{"exited", "missing"} {
		if err := m.PasteReview(id, "text"); !errors.Is(err, ErrPasteUnavailable) {
			t.Fatalf("session %q: %v", id, err)
		}
	}
	m.closed = true
	if err := m.PasteReview("missing", "text"); !errors.Is(err, ErrPasteUnavailable) {
		t.Fatal(err)
	}
}

type reviewPasteConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	mu     sync.Mutex
	input  bytes.Buffer
}

func (c *reviewPasteConn) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *reviewPasteConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.input.Write(b)
}
func (c *reviewPasteConn) Close() error {
	_ = c.writer.Close()
	return c.reader.Close()
}
func (c *reviewPasteConn) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.input.String()
}

// Proves the public native snapshot emits the mode this policy consumes, and
// the real asynchronous terminal input queue receives exactly one framed paste.
func TestReviewPasteNativeModeAndInputQueue(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 with the bundled native library")
	}
	r, w := io.Pipe()
	conn := &reviewPasteConn{reader: r, writer: w}
	term, err := mygoterm.New(mygoterm.Options{Conn: conn})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Feed([]byte("\x1b[?2004h"))
	if !bracketedPasteEnabled(term.Snapshot()) {
		t.Fatalf("native snapshot did not preserve bracketed mode: %q", term.Snapshot())
	}
	term.Send([]byte("existing input "))
	if err := pasteReview(term, "评论\nnext\n"); err != nil {
		t.Fatal(err)
	}
	want := "existing input \x1b[200~评论\nnext\n\x1b[201~"
	deadline := time.Now().Add(3 * time.Second)
	for !strings.HasSuffix(conn.text(), want) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 10)
	}
	if conn.text() != want {
		t.Fatalf("native input queue: %q", conn.text())
	}
	term.Feed([]byte("\x1b[?2004l"))
	if err := pasteReview(term, "must not send\n"); !errors.Is(err, ErrPasteNeedsDraft) {
		t.Fatal(err)
	}
	if conn.text() != want {
		t.Fatal("unbracketed mode received input")
	}
}
