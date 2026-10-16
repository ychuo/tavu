package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type outputBudget struct {
	mu        sync.Mutex
	remaining int64
	exceeded  bool
	cancel    context.CancelFunc
}

type boundedOutput struct {
	budget *outputBudget
	buffer bytes.Buffer
}

func (b *boundedOutput) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedOutput) String() string { return b.buffer.String() }

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.budget.mu.Lock()
	defer b.budget.mu.Unlock()
	n := len(p)
	if int64(n) > b.budget.remaining {
		b.buffer.Write(p[:int(b.budget.remaining)])
		b.budget.remaining = 0
		b.budget.exceeded = true
		b.budget.cancel()
	} else {
		b.buffer.Write(p)
		b.budget.remaining -= int64(n)
	}
	return n, nil
}

func (r *Repository) run(ctx context.Context, write bool, stdin io.Reader, limit int64, args ...string) ([]byte, error) {
	timeout := r.options.QueryTimeout
	if write {
		timeout = r.options.WriteTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if limit <= 0 {
		limit = r.options.MaxOutputBytes
	}
	budget := &outputBudget{remaining: limit, cancel: cancel}
	stdout, stderr := &boundedOutput{budget: budget}, &boundedOutput{budget: budget}
	argv := []string{"--literal-pathspecs", "-c", "color.ui=false", "-C", r.Root}
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	// An inherited GIT_DIR or alternate index must not redirect operations away
	// from the project the user opened. User configuration and PATH are retained.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX", "GIT_TERMINAL_PROMPT", "GIT_PAGER", "GIT_OPTIONAL_LOCKS":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")
	if !write {
		cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	cmd.WaitDelay = 500 * time.Millisecond
	configureProcess(cmd)
	err := cmd.Run()
	if budget.exceeded {
		return nil, ErrOutputLimit
	}
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return stdout.Bytes(), &CommandError{Args: append([]string(nil), args...), Stderr: stderr.String(), Stdout: stdout.String(), Cause: err}
	}
	return stdout.Bytes(), nil
}

func exitCode(err error) int {
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return -1
}
