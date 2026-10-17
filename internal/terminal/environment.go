package terminal

import (
	"os"
	"os/exec"
	"runtime"
)

// loginShellCommand clears color policy inherited from an app launcher only in
// the new interactive terminal. An automation host may disable its own output
// while this PTY supports truecolor. Shell startup files can still set their
// preferred policy. No process-global environment or forced-color override is
// used, and env execs the shell directly without an intermediate command string.
func loginShellCommand(configured string) ([]string, error) {
	shell := configured
	if shell == "" {
		if runtime.GOOS == "windows" {
			return nil, nil // Keep MyGo's native default-shell selection.
		}
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
			if runtime.GOOS == "darwin" {
				shell = "/bin/zsh"
			}
		}
	}
	path, err := exec.LookPath(shell)
	if err != nil {
		return nil, err
	}
	command := []string{path, "-l"}
	if runtime.GOOS == "windows" {
		return command, nil
	}
	prefix := []string{"/usr/bin/env"}
	for _, key := range []string{"NO_COLOR", "NODE_DISABLE_COLORS", "FORCE_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "TERM_PROGRAM_VERSION"} {
		prefix = append(prefix, "-u", key)
	}
	return append(prefix, command...), nil
}
