package views

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
)

func TestReviewCommentExpansionUpdatesAfterResizeWithoutExtraInput(t *testing.T) {
	a := testApp()
	body := strings.Repeat("words ", 24)
	cm, err := a.state.Review.AddFileComment("", fileCommentComparison(), body)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() {
			a.reviewCommentList(c, []review.Comment{cm})
		})
	}, 800, 400)
	for _, size := range []struct {
		width int
		more  bool
	}{{800, false}, {200, true}, {800, false}, {200, true}} {
		// SetSize processes only the resize and frames the application itself
		// requests. A manual Frame or pointer event would mask the regression.
		tt.SetSize(size.width, 400)
		if _, more := tt.Find("Expand Comment " + cm.ID); more != size.more {
			t.Fatalf("width %d: expansion control visible=%v, want %v", size.width, more, size.more)
		}
		if tt.FramePending() {
			t.Fatalf("width %d: comment layout continuously requests redraws", size.width)
		}
	}
	if a.state.Review.Comments[0].Body != body || len(a.documents) != 0 {
		t.Fatal("comment layout changed the saved text or opened a document")
	}
}

func TestReviewCommentActionsStayClickableBesideScrollbar(t *testing.T) {
	for _, width := range []int{200, 201, 400} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			a := testApp()
			a.commentDraft = "Keep this unsent comment"
			for i := range 30 {
				if _, err := a.state.Review.AddFileComment("", fileCommentComparison(), fmt.Sprintf("Review note %d", i+1)); err != nil {
					t.Fatal(err)
				}
			}
			first := a.state.Review.Comments[0]
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(appTheme())
				// Match Review's padded container and the list's reclaimed
				// horizontal space, using the actual production rows and menu.
				ui.Column(c).Fill().Padding(0, reviewContentInset).Children(func() {
					a.reviewCommentList(c, a.state.Review.Comments).Margin(0, -reviewContentInset)
				})
			}, width, 200)
			list, ok := tt.Find("Review Comments")
			if !ok {
				t.Fatal("review list is missing")
			}
			body, ok := tt.Find("Comment Body " + first.ID)
			if !ok {
				t.Fatal("first comment is missing")
			}
			menuLabel := "Comment Actions " + first.ID
			if _, visible := tt.Find(menuLabel); visible {
				t.Fatal("comment actions are visible before hovering the row")
			}
			tt.Move(body.X+8, body.Y+body.H/2)
			hoveredBody, _ := tt.Find("Comment Body " + first.ID)
			if hoveredBody.X != body.X || hoveredBody.W != body.W {
				t.Fatalf("hovering steals room from the comment: before=%+v after=%+v", body, hoveredBody)
			}
			menu, ok := tt.Find(menuLabel)
			if !ok {
				t.Fatal("hovering did not reveal comment actions")
			}
			// Click the last pixel of the actual button, where an overlaid
			// scrollbar used to intercept the press before it reached the menu.
			tt.ClickAt(menu.X+menu.W-1, menu.Y+menu.H/2)
			if err := tt.ChooseMenuItem("Edit"); err != nil {
				t.Fatalf("comment menu right edge is not clickable: %v", err)
			}
			if a.editingComment != first.ID || a.commentEditDraft != first.Body {
				t.Fatal("Edit did not open the selected comment draft")
			}
			if a.commentDraft != "Keep this unsent comment" || len(a.documents) != 0 || len(a.state.Review.Comments) != 30 || a.state.Review.Comments[0].Body != first.Body || a.state.Review.Comments[0].Resolved {
				t.Fatal("opening Edit changed the comment, composer, or current document")
			}
			if top, _ := a.commentList.Visible(); top != 0 {
				t.Fatal("clicking comment actions scrolled the list")
			}
			// The separately reserved outer gutter must still behave as a
			// scrollbar; protecting the actions must not disable its hit target.
			x := list.X + list.W - 3 - appTheme().ScrollbarWidth/2
			tt.Press(x, list.Y+10)
			tt.Move(x, list.Y+list.H-10)
			tt.Release(x, list.Y+list.H-10)
			if top, _ := a.commentList.Visible(); top <= 0 {
				t.Fatal("dragging the separate scrollbar gutter did not scroll")
			}
			if a.editingComment != first.ID || a.commentEditDraft != first.Body || len(a.state.Review.Comments) != 30 {
				t.Fatal("dragging the scrollbar activated a comment action")
			}
		})
	}
}
