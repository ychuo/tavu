package views

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func commitFixture() *App {
	a := testApp()
	a.repo = &gitrepo.Repository{}
	a.projectCtx = context.Background()
	a.status = gitrepo.Status{Token: "displayed-scope", Changes: []gitrepo.Change{{Path: "staged.go", Kind: gitrepo.Staged}, {Path: "working.go", Kind: gitrepo.Unstaged}}}
	a.commitMessage = "A subject\n\nA detailed message"
	return a
}

func TestInlineCommitRequiresMessageAndKnownStagedScope(t *testing.T) {
	cases := []struct {
		name   string
		change func(*App)
	}{
		{"blank message", func(a *App) { a.commitMessage = " \n\t" }},
		{"unstaged only", func(a *App) { a.status.Changes = a.status.Changes[1:] }},
		{"unknown status", func(a *App) { a.status.Token = "" }},
		{"no repository", func(a *App) { a.repo = nil }},
		{"no project context", func(a *App) { a.projectCtx = nil }},
		{"busy", func(a *App) { a.busy = true }},
		{"opening project", func(a *App) { a.loading = true }},
		{"committing", func(a *App) { a.committing = true }},
		{"conflicts", func(a *App) {
			a.status.Changes = append(a.status.Changes, gitrepo.Change{Path: "conflict.go", Kind: gitrepo.Conflict})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := commitFixture()
			tc.change(a)
			if _, ok := a.commitRequest(); ok {
				t.Fatal("commit accepted an unavailable or unsafe staged scope")
			}
		})
	}
	a := commitFixture()
	request, ok := a.commitRequest()
	if !ok || request.token != "displayed-scope" || request.message != a.commitMessage || a.stagedCount() != 1 {
		t.Fatal("submission did not capture the displayed scope and full message")
	}
	a.status.Token = "changed-after-submission"
	if request.token != "displayed-scope" {
		t.Fatal("background status changes replaced the token of an in-flight commit")
	}
}

func TestInlineCommitShortcutIsScopedToMessageInput(t *testing.T) {
	a := commitFixture()
	a.commitMessage = ""
	var elsewhere, submitted string
	count := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Width(280).Gap(8).Children(func() {
			a.commitComposerView(c, func() { count++; submitted = a.commitMessage })
			ui.TextInput(c, &elsewhere).Label("Other Input").FillWidth()
		})
	}, 300, 400)
	click(t, tt, "Commit Message")
	tt.Type("subject\n\nbody")
	tt.Key(ui.Cmd, ui.KeyEnter)
	if count != 1 || submitted != "subject\n\nbody" {
		t.Fatalf("focused Cmd+Enter submitted %d times with message %q", count, submitted)
	}
	click(t, tt, "Other Input")
	tt.Key(ui.Cmd, ui.KeyEnter)
	if count != 1 {
		t.Fatal("Cmd+Enter outside the commit message triggered a commit")
	}
	a.status.Changes = a.status.Changes[1:]
	tt.Frame()
	click(t, tt, "Commit Message")
	tt.Key(ui.Cmd, ui.KeyEnter)
	click(t, tt, "Commit Staged Changes")
	if count != 1 {
		t.Fatal("an empty staged scope allowed keyboard or button submission")
	}
}

func TestInlineCommitRejectsStatusChangedAfterLastVisibleFrame(t *testing.T) {
	for _, input := range []string{"button", "keyboard"} {
		t.Run(input, func(t *testing.T) {
			a := commitFixture()
			submitted := 0
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Width(280).Children(func() {
					a.commitComposerView(c, func() { submitted++ })
				})
			}, 300, 400)
			if input == "keyboard" {
				click(t, tt, "Commit Message")
			} else {
				button, _ := tt.Find("Commit Staged Changes")
				tt.Press(button.X+button.W/2, button.Y+button.H/2)
			}
			// A background query publishes a different index after the last
			// visible frame, before the already-targeted input is processed.
			a.status.Token = "new-unseen-scope"
			a.status.Changes[0].Path = "different-staged-file.go"
			if input == "keyboard" {
				tt.Key(ui.Cmd, ui.KeyEnter)
			} else {
				button, _ := tt.Find("Commit Staged Changes")
				tt.Release(button.X+button.W/2, button.Y+button.H/2)
			}
			if submitted != 0 || a.commitError != gitrepo.ErrStale.Error() || a.commitMessage == "" {
				t.Fatal("an old UI event submitted newly arrived staged scope or lost the draft")
			}
			// Once the updated scope is visible, a new explicit action can use it.
			tt.Frame()
			click(t, tt, "Commit Staged Changes")
			if submitted != 1 {
				t.Fatal("a new action could not submit the now-visible staged scope")
			}
		})
	}
}

func TestInlineCommitPreservesFailedDraftAndClearsSuccess(t *testing.T) {
	a := commitFixture()
	request, _ := a.commitRequest()
	a.busy, a.committing = true, true
	hookError := errors.New("pre-commit hook failed\nPlease fix the reported checks")
	a.finishCommit(request, gitrepo.Status{}, hookError)
	if a.commitMessage != request.message || a.commitError != hookError.Error() || !a.commitErrorExpanded || a.busy || a.committing {
		t.Fatal("a failed commit lost its draft or original error")
	}
	a.finishCommit(request, gitrepo.Status{}, gitrepo.ErrStale)
	if a.commitMessage != request.message || a.commitError != gitrepo.ErrStale.Error() {
		t.Fatal("a stale-scope rejection lost the user's message")
	}
	a.finishCommit(request, gitrepo.Status{HEAD: "committed-head"}, nil)
	if a.commitMessage != "" || a.commitError != "" || a.status.HEAD != "committed-head" {
		t.Fatal("a successful commit did not reset the composer")
	}
}

func TestInlineCommitDraftsAndCompletionsStayWithinProject(t *testing.T) {
	a := commitFixture()
	a.root = "/project-one"
	request, _ := a.commitRequest()
	a.switchCommitDraft("/project-two")
	a.root = "/project-two"
	a.generation++
	if a.commitMessage != "" {
		t.Fatal("a project inherited another project's commit draft")
	}
	a.commitMessage = "Second project draft"
	a.finishCommit(request, gitrepo.Status{HEAD: "old-completion"}, nil)
	if a.commitMessage != "Second project draft" || a.status.HEAD == "old-completion" {
		t.Fatal("an old project completion changed the new project's composer")
	}
	a.switchCommitDraft("/project-one")
	a.root = "/project-one"
	if a.commitMessage != request.message {
		t.Fatal("returning to a project did not restore its own draft")
	}
}

func TestInlineCommitLongErrorIsBoundedAndSnapshotHasNoComposer(t *testing.T) {
	a := commitFixture()
	a.commitError = strings.Repeat("A long original Git hook failure with details.\n", 100)
	a.commitErrorExpanded = true
	a.sidebarWidth = 280
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(256).Gap(8).Children(func() {
			a.commitComposerView(c, func() {})
			ui.Text(c, "After composer")
		})
	}, 280, 420)
	after, ok := tt.Find("After composer")
	if !ok || after.Y+after.H > 320 {
		t.Fatalf("long errors expanded the composer beyond its bounded area: %+v", after)
	}
	a.reviewTab = 1
	tt.Frame()
	if _, ok := tt.Find("Commit Message"); ok {
		t.Fatal("a historical snapshot showed a composer for the current Git index")
	}
}
