package issues

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/comments"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"math"
	"strings"
	"time"
)

type issueView struct {
	app.View
	api               jira.Api
	bottomBar         *app.ActionBar
	topBar            *app.ActionBar
	fuzzyFind         *app.FuzzyFind
	issue             *jira.Issue
	goBackFn          func()
	descriptionLimitX int
	descriptionLimitY int
	scrollY           int
	descriptionLines  int
	commentsLines     int
	detailsLines      int
	maxScrollY        int
	body              string
	detailRows        []detailRow
	detailLabelWidth  int
	summaryLen        int
	labels            string
	labelsLen         int
	comments          []comments.Comment
	lastY             int
	screenY           int
	boxTitleStyle     tcell.Style
	defaultStyle      tcell.Style
	dimStyle          tcell.Style
}

var (
	issueNavItems = []ui.NavItemConfig{
		ui.NavItemConfig{Action: ui.ActionStatusChange, Text1: ui.MessageChangeStatus, Text2: "[s]", Rune: 's'},
		ui.NavItemConfig{Action: ui.ActionAssigneeChange, Text1: ui.MessageAssignUser, Text2: "[a]", Rune: 'a'},
		ui.NavItemConfig{Action: ui.ActionComment, Text1: ui.MessageComment, Text2: "[c]", Rune: 'c'},
		ui.NavItemConfig{Action: ui.ActionEditDescription, Text1: "Edit description ", Text2: "[d]", Rune: 'd'},
		ui.NavItemConfig{Action: ui.ActionAddLabel, Text1: ui.MessageLabel, Text2: "[l]", Rune: 'l'},
		ui.NavItemConfig{Action: ui.ActionCreateIssue, Text1: ui.MessageCreateIssue, Text2: "[F6]", Key: tcell.KeyF6},
		ui.NavItemConfig{Action: ui.ActionOpen, Text1: ui.MessageOpen, Text2: "[o]", Rune: 'o'},
	}
)

const (
	maxCommentLineWidth = 150
	labelsDelimiter     = " | "
)

// detailRow is one line inside the Details box: an aligned "label   value",
// optionally followed by a dimmer parenthetical (dimValue). For timestamp rows
// value is the relative time ("2 hours ago") and dimValue the absolute date
// ("4 Jun 2026 10:35 AM +0200"); other rows leave dimValue empty.
type detailRow struct {
	label    string
	value    string
	dimValue string
}

// buildDetailRows returns the fixed set of rows shown in the issue's Details
// box: priority, type, and created/updated timestamps. The row count is
// constant so the surrounding scroll math (view.detailsLines) never drifts
// between Draw and Resize; empty values render blank rather than being
// dropped. now is injected so relative-time rendering stays deterministic.
func buildDetailRows(issue *jira.Issue, now time.Time) []detailRow {
	return []detailRow{
		{label: ui.MessageDetailPriority, value: issue.Fields.Priority.Name},
		{label: ui.MessageDetailType, value: issue.Fields.Type.Name},
		{
			label:    ui.MessageDetailCreated,
			value:    app.FormatRelativeTime(issue.Fields.Created, now),
			dimValue: app.FormatAbsoluteTime(issue.Fields.Created),
		},
		{
			label:    ui.MessageDetailUpdated,
			value:    app.FormatRelativeTime(issue.Fields.Updated, now),
			dimValue: app.FormatAbsoluteTime(issue.Fields.Updated),
		},
	}
}

// detailLabelWidth returns the widest label so values line up in a column.
func detailLabelWidth(rows []detailRow) int {
	labelWidth := 0
	for _, r := range rows {
		if len(r.label) > labelWidth {
			labelWidth = len(r.label)
		}
	}
	return labelWidth
}

func NewIssueView(issue *jira.Issue, goBackFn func(), api jira.Api) app.View {
	bottomBar := ui.CreateBottomActionBarWithItems(issueNavItems)
	bottomBar.AddItem(ui.CreateScrollBarItem())
	bottomBar.AddItem(ui.NewCancelBarItem())

	issueActionBar := ui.CreateIssueTopBar(issue)
	// The top bar already carries key/reporter/assignee/type/status; append a
	// relative "Updated" so freshness is visible at a glance. Formatting lives
	// here (not in ui.CreateIssueTopBar) because app.FormatRelativeTime is the
	// shared helper and this keeps the shared top-bar builder untouched.
	issueActionBar.AddItem(ui.NewAppTopBarItem(&ui.NavItemConfig{
		Text1: ui.MessageLabelUpdated,
		Text2: app.ActionBarLabel(app.FormatRelativeTime(issue.Fields.Updated, time.Now())),
	}))
	cs := comments.ParseCommentsFromIssue(issue, 1000, 1000)
	ls := strings.Join(issue.Fields.Labels, labelsDelimiter)
	labelsLen := len(ls)
	detailRows := buildDetailRows(issue, time.Now())

	return &issueView{
		api:              api,
		bottomBar:        bottomBar,
		topBar:           issueActionBar,
		issue:            issue,
		scrollY:          0,
		body:             issue.Fields.Description,
		comments:         cs,
		labels:           ls,
		labelsLen:        labelsLen,
		detailRows:       detailRows,
		detailLabelWidth: detailLabelWidth(detailRows),
		summaryLen:       len(issue.Fields.Summary),
		goBackFn:         goBackFn,
		boxTitleStyle:    app.DefaultStyle().Foreground(app.Color("details.foreground")),
		defaultStyle:     app.DefaultStyle(),
		// Absolute date reuses the box-header color (details.foreground, a dim
		// gray) so it recedes behind the brighter relative time. foreground2 is
		// an emphasis (brighter) color, not a muted one — wrong direction here.
		dimStyle: app.DefaultStyle().Foreground(app.Color("details.foreground")),
	}
}

func (view *issueView) Init() {
	go view.handleIssueAction()
}

func (view *issueView) Destroy() {
}

func (view *issueView) Draw(screen tcell.Screen) {
	if view.fuzzyFind == nil {
		app.DrawBox(screen, 1, 2-view.scrollY, view.summaryLen+4, 4-view.scrollY, view.boxTitleStyle)
		app.DrawText(screen, 2, 2-view.scrollY, view.boxTitleStyle, ui.MessageSummary)
		app.DrawText(screen, 3, 3-view.scrollY, view.defaultStyle, view.issue.Fields.Summary)

		view.lastY = 2 - view.scrollY + 2

		if view.labels != "" {
			app.DrawBox(screen, 1, view.lastY+1, view.labelsLen+4, view.lastY+3, view.boxTitleStyle)
			app.DrawText(screen, 2, view.lastY+1, view.boxTitleStyle, ui.MessageLabels)
			app.DrawTextLimited(screen, 3, view.lastY+2, view.descriptionLimitX, view.lastY+2, view.defaultStyle, view.labels)
			view.lastY = view.lastY + 3
		}

		// Details box: title row (lastY+1) + one row per detail starting at
		// lastY+2, with the bottom border at lastY+detailsLines so content sits
		// strictly inside it (detailsLines = len(rows)+2, set in Resize).
		// view.detailsLines is the single source of truth shared with Resize's
		// maxScrollY math so Draw and scroll never drift.
		app.DrawBox(screen, 1, view.lastY+1, view.descriptionLimitX+4, view.lastY+view.detailsLines, view.boxTitleStyle)
		app.DrawText(screen, 2, view.lastY+1, view.boxTitleStyle, ui.MessageDetails)
		for i, row := range view.detailRows {
			// Label column (padded) + value in the default style; the absolute
			// date, when present, follows in a dimmer style so the human-readable
			// relative time reads as primary.
			left := fmt.Sprintf("%-*s  %s", view.detailLabelWidth, row.label, row.value)
			app.DrawText(screen, 3, view.lastY+2+i, view.defaultStyle, left)
			if row.dimValue != "" {
				app.DrawText(screen, 3+len(left)+1, view.lastY+2+i, view.dimStyle, "("+row.dimValue+")")
			}
		}
		view.lastY = view.lastY + view.detailsLines + 1

		app.DrawBox(screen, 1, view.lastY+1, view.descriptionLimitX+4, view.lastY+1+view.descriptionLines+4, view.boxTitleStyle)
		app.DrawText(screen, 2, view.lastY+1, view.boxTitleStyle, ui.MessageDescription)
		app.DrawTextLimited(screen, 3, view.lastY+2, view.descriptionLimitX, view.descriptionLimitY, view.defaultStyle, view.body)

		view.lastY = view.lastY + view.descriptionLines + 6

		for _, comment := range view.comments {
			app.DrawBox(screen, 1, view.lastY+1, view.descriptionLimitX+4, view.lastY+1+comment.Lines+2, view.boxTitleStyle)
			app.DrawText(screen, 2, view.lastY+1, view.boxTitleStyle, comment.Title)
			app.DrawTextLimited(screen, 3, view.lastY+2, view.descriptionLimitX, view.descriptionLimitY, view.defaultStyle, comment.Body)
			view.lastY = view.lastY + 1 + comment.Lines + 3
		}
	}
	view.bottomBar.Draw(screen)
	view.topBar.Draw(screen)
	if view.fuzzyFind != nil {
		view.fuzzyFind.Draw(screen)
	}
}

func (view *issueView) Update() {
	view.bottomBar.Update()
	view.topBar.Update()
	if view.fuzzyFind != nil {
		view.fuzzyFind.Update()
	}
}

func (view *issueView) Resize(screenX, screenY int) {
	view.screenY = screenY
	view.descriptionLimitX = app.ClampInt(int(math.Floor(float64(screenX)*0.9)), 1, 10000)
	view.descriptionLimitY = 1000
	view.descriptionLines = app.DrawTextLimited(nil, 0, 0, view.descriptionLimitX, view.descriptionLimitY, view.defaultStyle, view.body) + 1
	commentsLines := 0
	view.comments = comments.ParseCommentsFromIssue(view.issue, view.descriptionLimitX, view.descriptionLimitY)
	for _, comment := range view.comments {
		commentsLines = commentsLines + comment.Lines + 3
	}
	view.commentsLines = commentsLines + len(view.comments) + 1
	// Details box row span: title + one row per detail + bottom border.
	// Single source of truth shared with Draw (see Draw's Details section).
	view.detailsLines = len(view.detailRows) + 2
	topAndBottomBarSize := 12
	// maxScrollY is the content height beyond the viewport (the existing
	// heuristic), plus a screenY/3 buffer so scrolling to the end lands on an
	// obviously-empty region — making "no more content" unmistakable. Larger
	// scrollY pushes content up, so the buffer enlarges the cap; it can never
	// hide the last line (that's reached before the cap).
	scrollBuffer := screenY / 3
	view.maxScrollY = app.ClampInt(int(math.Abs(float64(screenY-topAndBottomBarSize-view.descriptionLines-view.commentsLines-view.detailsLines-10)))+scrollBuffer, 0, 2000)
	view.bottomBar.Resize(screenX, screenY)
	view.topBar.Resize(screenX, screenY)
	if view.fuzzyFind != nil {
		view.fuzzyFind.Resize(screenX, screenY)
	}
}

func (view *issueView) HandleKeyEvent(ev *tcell.EventKey) {
	view.bottomBar.HandleKeyEvent(ev)
	view.topBar.HandleKeyEvent(ev)
	if view.fuzzyFind != nil {
		view.fuzzyFind.HandleKeyEvent(ev)
	}
	if ev.Key() == tcell.KeyUp || ev.Key() == tcell.KeyTab {
		view.scrollY = app.ClampInt(view.scrollY-1, 0, view.maxScrollY)
	}
	if ev.Key() == tcell.KeyDown || ev.Key() == tcell.KeyBacktab {
		view.scrollY = app.ClampInt(view.scrollY+1, 0, view.maxScrollY)
	}
	if ev.Key() == tcell.KeyPgUp {
		pageSize := view.calculatePageSize()
		view.scrollY = app.ClampInt(view.scrollY-pageSize, 0, view.maxScrollY)
	}
	if ev.Key() == tcell.KeyPgDn {
		pageSize := view.calculatePageSize()
		view.scrollY = app.ClampInt(view.scrollY+pageSize, 0, view.maxScrollY)
	}
}

// calculatePageSize returns how many rows PgUp/PgDn should advance scrollY.
// Visible content height is screen height minus chrome (top/bottom bars + a
// margin), capped at half the screen so a page-jump never feels disorienting,
// and floored at 5 so very small terminals still scroll noticeably.
func (view *issueView) calculatePageSize() int {
	const chromeRows = 16 // top/bottom bars (12) + margin (4)
	const minPage = 5
	visibleHeight := view.screenY - chromeRows
	pageSize := app.ClampInt(visibleHeight, 1, view.screenY/2)
	if pageSize < minPage {
		pageSize = minPage
	}
	return pageSize
}

func (view *issueView) goBack() {
	if view.goBackFn != nil {
		view.goBackFn()
	}
}

func (view *issueView) handleIssueAction() {
	if selectedAction := <-view.bottomBar.Action; true {
		switch selectedAction {
		case ui.ActionCancel:
			view.goBack()
			return
		case ui.ActionStatusChange:
			app.GoTo("status-change", view.issue, view.reopen, view.api)
			return
		case ui.ActionAssigneeChange:
			app.GoTo("users-assign", view.issue, view.reopen, view.api)
			return
		case ui.ActionComment:
			app.GoTo("text-writer", &ui.TextWriterArgs{
				Header: ui.MessageTypeCommentAndSave,
				GoBack: func() {
					view.reopen()
				},
				TextConsumer: func(s string) {
					view.doComment(view.issue, s)
				},
				MaxLength: maxCommentLineWidth,
			})
			return
		case ui.ActionEditDescription:
			app.GoTo("text-writer", &ui.TextWriterArgs{
				Header: ui.MessageTypeDescriptionAndSave,
				GoBack: func() {
					view.reopen()
				},
				TextConsumer: func(s string) {
					view.doUpdateDescription(view.issue, s)
				},
				MaxLength:   1000,
				InitialText: view.issue.Fields.Description,
			})
			return
		case ui.ActionAddLabel:
			app.GoTo("labels-add", view.issue, view.reopen, view.api)
			return
		case ui.ActionCreateIssue:
			projectId := ""
			if view.issue != nil {
				projectId = view.issue.Fields.Project.Id
			}
			ui.OpenCreateIssueInBrowser(view.api, projectId, 0)
			go view.handleIssueAction()
			return
		case ui.ActionOpen:
			OpenIssueInBrowser(view.issue, view.api)
			go view.handleIssueAction()
			return
		}
	}
}

func (view *issueView) reopen() {
	app.GoTo("issue", view.issue.Key, view.goBackFn, view.api)
}

func (view *issueView) doComment(issue *jira.Issue, comment string) {
	app.GetApp().LoadingWithText(true, ui.MessageAddingComment)
	err := view.api.DoComment(issue.Key, comment)
	app.GetApp().Loading(false)
	if err != nil {
		app.Error(fmt.Sprintf(ui.MessageCannotAddComment, issue.Key, err))
	}
	app.Success(fmt.Sprintf(ui.MessageCommentSuccess, issue.Key))
}

func (view *issueView) doUpdateDescription(issue *jira.Issue, description string) {
	app.GetApp().LoadingWithText(true, "Updating description")
	err := view.api.DoUpdateDescription(issue.Key, description)
	app.GetApp().Loading(false)
	if err != nil {
		app.Error(fmt.Sprintf("Cannot update description for %s. Reason: %s", issue.Key, err))
		return
	}
	app.Success(fmt.Sprintf("Description updated successfully for %s", issue.Key))
}
