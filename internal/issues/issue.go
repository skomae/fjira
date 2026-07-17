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
	relatedRows       []string
	relatedKeys       []string
	relatedColStart   int
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
		ui.NavItemConfig{Action: ui.ActionJumpToRelated, Text1: ui.MessageJumpToRelated, Text2: "[j]", Rune: 'j'},
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

// buildDetailRows returns the rows shown in the issue's Details box: the
// parent/epic link (when set), priority, type, and created/updated timestamps.
// The row count is derived from the returned slice (see view.detailsLines /
// detailLabelWidth), so an omitted parent row keeps the scroll math correct.
// Empty values render blank rather than being dropped; now is injected so
// relative-time rendering stays deterministic.
func buildDetailRows(issue *jira.Issue, now time.Time) []detailRow {
	rows := make([]detailRow, 0, 5)
	if row, ok := parentDetailRow(issue); ok {
		rows = append(rows, row)
	}
	return append(rows,
		detailRow{label: ui.MessageDetailPriority, value: issue.Fields.Priority.Name},
		detailRow{label: ui.MessageDetailType, value: issue.Fields.Type.Name},
		detailRow{
			label:    ui.MessageDetailCreated,
			value:    app.FormatRelativeTime(issue.Fields.Created, now),
			dimValue: app.FormatAbsoluteTime(issue.Fields.Created),
		},
		detailRow{
			label:    ui.MessageDetailUpdated,
			value:    app.FormatRelativeTime(issue.Fields.Updated, now),
			dimValue: app.FormatAbsoluteTime(issue.Fields.Updated),
		},
	)
}

// parentDetailRow builds the Details row for the issue's parent link, and false
// when there is none. The parent is an epic (label "Epic") or a regular ticket
// (label "Parent"), distinguished by the parent's issue type. The human-readable
// summary is the primary value; the parent key follows in the dimmer style, e.g.
// "Epic   Auth Revamp (COINS-100)" or "Parent  Fix login (COINS-50)".
func parentDetailRow(issue *jira.Issue) (detailRow, bool) {
	parent := issue.Fields.Parent
	if parent.Key == "" {
		return detailRow{}, false
	}
	label := ui.MessageDetailParent
	if strings.EqualFold(parent.Fields.Type.Name, "Epic") {
		label = ui.MessageDetailEpic
	}
	return detailRow{
		label:    label,
		value:    parent.Fields.Summary,
		dimValue: parent.Key,
	}, true
}

const (
	// doneMark leads a related-ticket line when its status is in the "done"
	// category; notDoneMark is the same width in blank so keys stay aligned.
	doneMark    = "✓ "
	notDoneMark = "  "
)

// buildRelatedRows renders the right-column entries from the issue's own
// payload: its child tickets (subtasks) followed by its related/linked tickets.
// Each line is "<mark><KEY> <summary>" as a single string so it can be drawn
// (and clipped) in one pass — a done ticket gets a leading ✓. The parallel keys
// slice holds each row's bare issue key so the jump modal (see runJumpToRelated)
// can open the selected ticket without parsing it back out of the display line.
// Epic children are NOT here (they aren't in the payload); those are fetched
// separately, see loadEpicChildren. Returns nil,nil when there are none.
func buildRelatedRows(issue *jira.Issue) (rows []string, keys []string) {
	n := len(issue.Fields.Subtasks) + len(issue.Fields.IssueLinks)
	rows = make([]string, 0, n)
	keys = make([]string, 0, n)
	for i := range issue.Fields.Subtasks {
		st := &issue.Fields.Subtasks[i]
		rows = append(rows, relatedLine(st.Key, st.Fields.Summary, st.Fields.Status))
		keys = append(keys, st.Key)
	}
	for _, link := range issue.Fields.IssueLinks {
		if ref := link.Linked(); ref != nil {
			rows = append(rows, relatedLine(ref.Key, ref.Fields.Summary, ref.Fields.Status))
			keys = append(keys, ref.Key)
		}
	}
	return rows, keys
}

// relatedRowsFromIssues renders right-column entries from full issues returned
// by a search (e.g. an epic's children via `parent = KEY`). Returns the display
// rows and the parallel bare-key slice (see buildRelatedRows).
func relatedRowsFromIssues(issues []jira.Issue) (rows []string, keys []string) {
	rows = make([]string, 0, len(issues))
	keys = make([]string, 0, len(issues))
	for i := range issues {
		it := &issues[i]
		rows = append(rows, relatedLine(it.Key, it.Fields.Summary, it.Fields.Status))
		keys = append(keys, it.Key)
	}
	return rows, keys
}

// relatedLine formats one related ticket as "<mark><KEY> <summary>", with a
// leading ✓ when its status is in the "done" category.
func relatedLine(key, summary string, status jira.Status) string {
	mark := notDoneMark
	if status.IsDone() {
		mark = doneMark
	}
	return fmt.Sprintf("%s%s %s", mark, key, summary)
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
	relatedRows, relatedKeys := buildRelatedRows(issue)

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
		relatedRows:      relatedRows,
		relatedKeys:      relatedKeys,
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
	// An epic's children are not in its own payload (unlike sub-tasks), so fetch
	// them with a deferred `parent = KEY` query after the view is already loaded
	// and rendered. Fire only when the payload carries no sub-tasks: a story's
	// sub-tasks already come back via `parent = KEY` too, so gating on empty
	// sub-tasks both avoids double-listing them and skips the call for leaves.
	if len(view.issue.Fields.Subtasks) == 0 && view.issue.Key != "" {
		go view.loadEpicChildren()
	}
}

// loadEpicChildren runs the deferred `parent = KEY` search off the UI thread,
// then marshals the result onto the app loop via RunOnAppRoutine so the state
// mutation runs on the render goroutine (see applyEpicChildren). A late result
// after navigating away harmlessly recomputes/redraws a detached view.
func (view *issueView) loadEpicChildren() {
	defer app.GetApp().PanicRecover()
	children, err := view.api.SearchJql(fmt.Sprintf("parent = \"%s\" ORDER BY key ASC", view.issue.Key))
	if err != nil || len(children) == 0 {
		return
	}
	rows, keys := relatedRowsFromIssues(children)
	app.GetApp().RunOnAppRoutine(func() { view.applyEpicChildren(rows, keys) })
}

// applyEpicChildren appends fetched epic-children rows (and their parallel keys,
// keeping relatedRows/relatedKeys aligned for the jump modal) and reflows the
// Details box. Enqueued via RunOnAppRoutine so it runs on the render goroutine;
// the relatedRows write joins the same ambient Resize/Draw sharing the rest of
// the view already relies on (no view-layer locks anywhere).
func (view *issueView) applyEpicChildren(rows []string, keys []string) {
	view.relatedRows = append(view.relatedRows, rows...)
	view.relatedKeys = append(view.relatedKeys, keys...)
	view.recomputeDetailsLayout()
	app.GetApp().SetDirty()
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

		// Details box. The left column holds the metadata rows; the right column
		// (when the issue has children/links) lists related tickets. The box is
		// as tall as the taller column: detailsLines = max(left, right)+2, the
		// single source of truth shared with Resize's maxScrollY math, so Draw
		// and scroll never drift. Content sits at rows lastY+2.., bottom border
		// at lastY+detailsLines. Both columns clip so long text can't bleed.
		app.DrawBox(screen, 1, view.lastY+1, view.descriptionLimitX+4, view.lastY+view.detailsLines, view.boxTitleStyle)
		app.DrawText(screen, 2, view.lastY+1, view.boxTitleStyle, ui.MessageDetails)
		leftLimit := view.leftColLimit()
		for i, row := range view.detailRows {
			// Label column (padded) + value in the default style; the absolute
			// date, when present, follows in a dimmer style so the human-readable
			// relative time reads as primary. Both parts clip at the left-column
			// edge so they never spill into the related column.
			left := fmt.Sprintf("%-*s  %s", view.detailLabelWidth, row.label, row.value)
			app.DrawTextLimited(screen, 3, view.lastY+2+i, leftLimit, view.lastY+2+i, view.defaultStyle, left)
			if row.dimValue != "" {
				dimX := 3 + len(left) + 1
				if dimX <= leftLimit {
					app.DrawTextLimited(screen, dimX, view.lastY+2+i, leftLimit, view.lastY+2+i, view.dimStyle, "("+row.dimValue+")")
				}
			}
		}
		if len(view.relatedRows) > 0 {
			app.DrawText(screen, view.relatedColStart, view.lastY+1, view.boxTitleStyle, ui.MessageDetailRelated)
			for i, line := range view.relatedRows {
				// One clipped pass per line — the leading ✓ is multibyte, so we
				// never index past it; the whole "<mark><KEY> <summary>" is drawn
				// and truncated at the box's right inner edge.
				app.DrawTextLimited(screen, view.relatedColStart, view.lastY+2+i, view.descriptionLimitX+2, view.lastY+2+i, view.defaultStyle, line)
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

// leftColLimit is the rightmost x the metadata (left) column may draw to. When
// a related column is present it stops one cell short of it; otherwise it uses
// the box's full inner width.
func (view *issueView) leftColLimit() int {
	if len(view.relatedRows) > 0 {
		return view.relatedColStart - 2
	}
	return view.descriptionLimitX + 2
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
	view.recomputeDetailsLayout()
	view.bottomBar.Resize(screenX, screenY)
	view.topBar.Resize(screenX, screenY)
	if view.fuzzyFind != nil {
		view.fuzzyFind.Resize(screenX, screenY)
	}
}

// recomputeDetailsLayout derives every layout value that depends on the related
// rows, from the already-computed line counts and screen size, so the Details
// box height and scroll cap always match the current related rows. Callers:
// Resize (on the events goroutine) and applyEpicChildren (on the render loop) —
// the same ambient, lock-free view-state sharing Resize/Draw already use.
func (view *issueView) recomputeDetailsLayout() {
	// Related column starts a little past the box midpoint so the metadata
	// column keeps its natural (usually wider) share. Only meaningful when
	// there are related rows to draw.
	view.relatedColStart = 3 + (view.descriptionLimitX*55)/100
	// Details box row span: title + max(metadata rows, related rows) + bottom
	// border. The box is as tall as the taller column. Single source of truth
	// shared with Draw and the maxScrollY math below.
	view.detailsLines = app.MaxInt(len(view.detailRows), len(view.relatedRows)) + 2
	const topAndBottomBarSize = 12
	// maxScrollY is the content height beyond the viewport (the existing
	// heuristic), plus a screenY/3 buffer so scrolling to the end lands on an
	// obviously-empty region — making "no more content" unmistakable. Larger
	// scrollY pushes content up, so the buffer enlarges the cap; it can never
	// hide the last line (that's reached before the cap).
	scrollBuffer := view.screenY / 3
	view.maxScrollY = app.ClampInt(int(math.Abs(float64(view.screenY-topAndBottomBarSize-view.descriptionLines-view.commentsLines-view.detailsLines-10)))+scrollBuffer, 0, 2000)
}

func (view *issueView) HandleKeyEvent(ev *tcell.EventKey) {
	// When the jump modal is open it owns the keyboard: forward keystrokes only
	// to it and skip the action bars and scrolling. Otherwise typing the query
	// (letters, arrows, Tab) would leak through — re-firing the [j] action or
	// scrolling the issue underneath the modal.
	if view.fuzzyFind != nil {
		view.fuzzyFind.HandleKeyEvent(ev)
		return
	}
	view.bottomBar.HandleKeyEvent(ev)
	view.topBar.HandleKeyEvent(ev)
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
// Delegates to app.ScrollPageSize so paging matches the text editor.
func (view *issueView) calculatePageSize() int {
	return app.ScrollPageSize(view.screenY)
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
		case ui.ActionJumpToRelated:
			view.runJumpToRelated()
			return
		}
	}
}

// runJumpToRelated opens a fuzzy-find modal over the view, prefilled with this
// issue's related tickets (the same subtasks/links/epic-children shown in the
// Related column, keyed by relatedKeys). Selecting one navigates to it in the
// detail view; Esc dismisses the modal and re-arms the action handler. A no-op
// when there are no related tickets to jump to.
func (view *issueView) runJumpToRelated() {
	if len(view.relatedKeys) == 0 {
		go view.handleIssueAction()
		return
	}
	a := app.GetApp()
	// Snapshot the rows/keys so a late epic-children append (applyEpicChildren,
	// on the render goroutine) can't grow relatedRows out from under the index
	// the fuzzy find hands back — the modal shows the tickets present when it
	// opened.
	keys := make([]string, len(view.relatedKeys))
	copy(keys, view.relatedKeys)
	rows := make([]string, len(view.relatedRows))
	copy(rows, view.relatedRows)
	view.fuzzyFind = app.NewFuzzyFind(ui.MessageJumpToRelatedFuzzyFind, rows)
	// FuzzyFind.Draw self-initialises its screen size on first render, so no
	// explicit Resize is needed here; a later terminal resize is forwarded by
	// the view's own Resize (which guards on fuzzyFind != nil).
	if chosen := <-view.fuzzyFind.Complete; true {
		view.fuzzyFind = nil
		a.ClearNow()
		if chosen.Index >= 0 && chosen.Index < len(keys) {
			app.GoTo("issue", keys[chosen.Index], view.goBackFn, view.api)
			return
		}
		go view.handleIssueAction()
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
