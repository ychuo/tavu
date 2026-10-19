//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package gitrepo

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
