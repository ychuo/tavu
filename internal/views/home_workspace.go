package views

import (
	"context"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"tavu/internal/files"
	sessions "tavu/internal/terminal"
)

// This key is never a filesystem root and is never stored as a recent project.
const standaloneWorkspaceKey = "tavu:standalone"
const projectHomeSessionKey = "tavu:home"

func (a *App) workspaceKey() string {
	if a.standalone {
		return standaloneWorkspaceKey
	}
	return a.root
}

func (s *projectSessionState) workspaceKey() string {
	if s.standalone {
		return standaloneWorkspaceKey
	}
	return s.root
}

func (a *App) isProjectHome() bool { return a.root == "" && !a.standalone }

// A standalone workspace deliberately has no project filesystem, repository,
// directory watcher or explorer. Only explicitly opened documents are read.
func (a *App) activateStandalone() bool {
	if a.projectTransitionBlocked() {
		return false
	}
	if a.standalone {
		return true
	}
	a.ensureProjectSessions()
	s := a.projects[standaloneWorkspaceKey]
	if s == nil {
		s = a.emptyProjectSession()
		s.standalone = true
		a.projects[standaloneWorkspaceKey] = s
		a.projectOrder = append(a.projectOrder, standaloneWorkspaceKey)
	}
	a.activateProject(s)
	return true
}

func (a *App) openHomeTerminal() {
	if !a.activateStandalone() {
		return
	}
	a.newWorkspaceTerminal()
}

func (a *App) newWorkspaceTerminal() {
	if a.isProjectHome() && !a.activateStandalone() {
		return
	}
	if a.standalone && a.terms == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			a.fail(err)
			return
		}
		a.terms = a.newStandaloneTerminalManager(home)
	}
	a.newTerminal(false)
}

func (a *App) newStandaloneTerminalManager(home string) *sessions.Manager {
	return sessions.NewManager(sessions.Config{
		Dir: home, Shell: a.settings.Shell, Theme: a.settings.Theme,
		FontSize: a.settings.FontSize, FontFamily: a.settings.FontFamily,
		Dispatch: a.update, OnChange: func() {},
		OnError: func(err error) {
			if a.standalone {
				a.fail(err)
			}
		},
		OnLink: func(target sessions.LinkTarget) {
			if a.standalone {
				a.loadStandaloneFile(target.Path, target.Line, true, a.update)
			}
		},
	})
}

func (a *App) chooseHomeFile() {
	if a.workspaceModalOpen() || a.loading || a.quitting || a.busy || a.fileBusy {
		return
	}
	a.choosingProject = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: a.tr("Open File…")})
		a.update(func() {
			a.choosingProject = false
			a.fail(err)
			if len(paths) > 0 {
				a.openHomePath(paths[0])
			}
		})
	}()
}

func (a *App) openHomePath(path string) {
	if a.projectTransitionBlocked() {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		a.fail(err)
		return
	}
	if info.IsDir() {
		a.requestProject(path)
		return
	}
	a.openStandaloneFile(path, 1)
}

func (a *App) openStandaloneFile(path string, line int) {
	a.startStandaloneFileLoad(path, line, a.update)
}

func (a *App) startStandaloneFileLoad(path string, line int, publish func(func())) {
	a.loadStandaloneFile(path, line, false, publish)
}

func (a *App) loadStandaloneFile(path string, line int, navigate bool, publish func(func())) {
	// Validate the candidate before switching away from the existing workspace.
	absolute, service, err := standaloneFileSource(path)
	if err != nil {
		a.fail(err)
		return
	}
	if _, err := os.Lstat(absolute); err != nil {
		a.fail(err)
		return
	}
	if !a.activateStandalone() {
		return
	}
	a.startFileLoad(diskOpenRequest{path: absolute, service: service, servicePath: filepath.Base(absolute), line: line, navigate: navigate}, publish)
}

// A document's service keeps save/reload checks tied to its original directory
// identity. Projects continue to use their existing shared filesystem service.
func (a *App) documentFileService(d *document) (*files.Service, string) {
	if d == nil {
		return nil, ""
	}
	if d.fileService != nil {
		return d.fileService, d.fileServicePath
	}
	return a.fs, d.Path
}

func (a *App) showProjectHome() {
	if a.projectTransitionBlocked() {
		return
	}
	a.activateProjectHome()
}

func (a *App) activateProjectHome() {
	a.parkProject()
	a.restoreProjectSession(a.emptyProjectSession())
	a.settings.LastProject = ""
	a.saveSettings()
}

func (a *App) workspaceContext() context.Context {
	if a.projectCtx != nil {
		return a.projectCtx
	}
	return context.Background()
}

// Closing the last standalone tab returns to another retained workspace, or home.
func (a *App) finishEmptyStandalone() {
	if a.standalone && !a.termStarting && len(a.documents) == 0 && (a.terms == nil || len(a.terms.Sessions()) == 0) {
		a.closeProjectNow(standaloneWorkspaceKey)
	}
}
