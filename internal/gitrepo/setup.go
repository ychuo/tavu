package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Init(ctx context.Context, dir string) (*Repository, error) {
	abs, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	info, e := os.Stat(abs)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Choose a folder")
	}
	if existing, err := Open(ctx, abs); err == nil {
		return nil, fmt.Errorf("Already inside Git repository %s", existing.Root)
	} else if !errors.Is(err, ErrNotRepository) {
		return nil, err
	}
	r := &Repository{Root: abs, options: Options{}.defaults()}
	if _, e = r.run(ctx, true, nil, 0, "init", "--initial-branch=main"); e != nil {
		return nil, e
	}
	return Open(ctx, abs)
}
func (r *Repository) AddRemote(ctx context.Context, name, url string) error {
	r.writes.Lock()
	defer r.writes.Unlock()
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("Enter a valid remote name")
	}
	if _, e := r.run(ctx, false, nil, 0, "check-ref-format", "refs/remotes/"+name+"/branch"); e != nil {
		return e
	}
	url = strings.TrimSpace(url)
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsAny(url, "\x00\r\n") || strings.Contains(url, "::") {
		return fmt.Errorf("Enter an HTTPS, SSH, or local repository URL")
	}
	_, e := r.run(ctx, true, nil, 0, "remote", "add", "--", name, url)
	return e
}
