package ui

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/stretchr/testify/assert"
)

func TestTextWriterView(t *testing.T) {
	type args struct {
		args *TextWriterArgs
	}
	tests := []struct {
		name string
		args args
	}{
		{"should create new text writer view", args{args: &TextWriterArgs{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, NewTextWriterView(tt.args.args), "NewTextWriterView(%v)", tt.args)
		})
	}
}

func Test_fjiraTextWriterView_Destroy(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"should run Destroy without problem"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{})
			view.Destroy()
		})
	}
}

func Test_fjiraTextWriterView_Draw(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	type args struct {
		screen tcell.Screen
	}
	tests := []struct {
		name string
		args args
	}{
		{"should draw text writer view", args{screen: screen}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
			view.text = "Test text"

			// Initialize screen dimensions and update text lines
			x, y := tt.args.screen.Size()
			view.Resize(x, y)
			view.updateTextLines()

			// when
			view.Draw(tt.args.screen)
			var buffer bytes.Buffer
			contents, x, y := tt.args.screen.(tcell.SimulationScreen).GetContents()
			tt.args.screen.Show()
			for i := 0; i < x*y; i++ {
				if len(contents[i].Bytes) != 0 {
					buffer.Write(contents[i].Bytes)
				}
			}
			result := buffer.String()

			// then
			assert.Contains(t, result, view.text)
		})
	}
}

func Test_fjiraTextWriterView_HandleKeyEvent(t *testing.T) {
	type args struct {
		ev []*tcell.EventKey
	}
	tests := []struct {
		name            string
		args            args
		expectedComment string
	}{
		{"should handle key events and Write text", args{ev: []*tcell.EventKey{
			tcell.NewEventKey(0, 'a', tcell.ModNone),
			tcell.NewEventKey(0, 'b', tcell.ModNone),
			tcell.NewEventKey(0, 'c', tcell.ModNone),
		}}, "abc"},
		{"should handle key events with backspace", args{ev: []*tcell.EventKey{
			tcell.NewEventKey(0, 'a', tcell.ModNone),
			tcell.NewEventKey(0, 'b', tcell.ModNone),
			tcell.NewEventKey(0, 'c', tcell.ModNone),
			tcell.NewEventKey(tcell.KeyBackspace, '-', tcell.ModNone),
		}}, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)

			// when
			for _, key := range tt.args.ev {
				view.HandleKeyEvent(key)
			}

			// then
			assert.Equal(t, tt.expectedComment, view.text)
		})
	}
}

func Test_fjiraTextWriterView_PageUpDown(t *testing.T) {
	// PageUp/PageDown move the cursor by one page, where a page is the shared
	// app.ScrollPageSize amount so paging matches the issue detail view.
	const screenY = 40
	page := app.ScrollPageSize(screenY)

	// A document with plenty of lines so a page jump lands mid-document.
	lines := make([]string, page*3)
	for i := range lines {
		lines[i] = "line"
	}
	doc := strings.Join(lines, "\n")

	view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
	view.text = doc
	view.updateTextLines()
	view.Resize(80, screenY)

	// Start at the top.
	view.setCursorToLineCol(0, 0)
	view.updateDesiredCol()
	line, _ := view.getCursorLineCol()
	assert.Equal(t, 0, line, "precondition: cursor at top")

	// PageDown moves down by one page.
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	line, _ = view.getCursorLineCol()
	assert.Equal(t, page, line, "PageDown should move cursor down one page")

	// Another PageDown moves down another page.
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	line, _ = view.getCursorLineCol()
	assert.Equal(t, page*2, line, "second PageDown should move another page")

	// PageUp moves back up by one page.
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	line, _ = view.getCursorLineCol()
	assert.Equal(t, page, line, "PageUp should move cursor up one page")

	// PageUp past the top clamps to the first line.
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	line, _ = view.getCursorLineCol()
	assert.Equal(t, 0, line, "PageUp past the top should clamp to first line")

	// PageDown past the bottom clamps to the last line.
	for i := 0; i < 10; i++ {
		view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	}
	line, _ = view.getCursorLineCol()
	assert.Equal(t, len(lines)-1, line, "PageDown past the bottom should clamp to last line")
}

func Test_fjiraTextWriterView_TextConsumer(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()

	tests := []struct {
		name string
	}{
		{"should initialize text consumer handling"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			app.InitTestApp(screen)
			done := make(chan bool)
			consumer := func(str string) {
				done <- true
			}
			view := NewTextWriterView(&TextWriterArgs{
				TextConsumer: consumer,
			}).(*TextWriterView)

			// when
			view.Init()
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyF2, 'F', tcell.ModNone))

			// then
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fail()
			}
		})
	}
}

func Test_fjiraTextWriterView_GoBack(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()

	tests := []struct {
		name string
	}{
		{"should initialize go-back handling"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			app.InitTestApp(screen)
			done := make(chan bool)
			goBack := func() {
				done <- true
			}
			view := NewTextWriterView(&TextWriterArgs{
				GoBack: goBack,
			}).(*TextWriterView)

			// when
			view.Init()
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyEscape, 'E', tcell.ModNone))

			// then
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fail()
			}
		})
	}
}

func Test_fjiraTextWriterView_Resize(t *testing.T) {
	type args struct {
		screenX int
		screenY int
	}
	tests := []struct {
		name string
		args args
	}{
		{"should resize without problems", args{screenY: 10, screenX: 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
			view.Resize(tt.args.screenX, tt.args.screenY)
		})
	}
}

func Test_fjiraTextWriterView_Update(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"should update without problems"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
			view.Update()
		})
	}
}

func Test_fjiraTextWriterView_CursorGlyphMultiByte(t *testing.T) {
	// Regression: the glyph drawn under the cursor used to be byte-indexed into
	// the display string, so any multi-byte rune earlier on the line shifted the
	// index and a wrong (often mid-sequence) byte was rendered under the cursor.
	tests := []struct {
		name        string
		text        string
		cursorPos   int    // rune offset of the cursor within `text`
		expectGlyph string // glyph that should be drawn in the cursor cell
	}{
		{"ascii", "abcdef", 3, "d"},
		{"after 2-byte accent", "café x", 5, "x"},
		{"on the accent", "café x", 3, "é"},
		{"after em-dash", "a — b", 4, "b"},
		{"after emoji", "hi 🚀 go", 5, "g"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("utf-8")
			_ = screen.Init() //nolint:errcheck
			defer screen.Fini()

			view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
			view.text = tt.text
			view.cursorPos = tt.cursorPos
			view.updateTextLines()

			sx, sy := screen.Size()
			view.Resize(sx, sy)
			view.Draw(screen)
			screen.Show()

			// Cursor is on the first (only) line, no horizontal scroll, so the
			// cursor cell is at x = 2 + cursorPos, y = textStartY.
			cursorX := 2 + tt.cursorPos
			glyph, _, _ := screen.Get(cursorX, view.textStartY)
			assert.Equal(t, tt.expectGlyph, glyph, "glyph under cursor for %q at col %d", tt.text, tt.cursorPos)
		})
	}
}

func Test_fjiraTextWriterView_CursorGlyphHorizontalScroll(t *testing.T) {
	// A line wider than the text area scrolls horizontally (leftTruncated),
	// which routes the cursor glyph through getCursorScreenX's scroll-offset
	// branch. With a multi-byte rune earlier on the line, the byte-index bug
	// mis-picked the glyph here too. Locate the reverse-video cursor cell and
	// verify its glyph.
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()

	// 76-cell text area on the default 80x25 screen; build a much longer line
	// with a multi-byte rune near the start so the byte/rune offset diverges.
	line := "é" + strings.Repeat("a", 60) + "TARGETx" + strings.Repeat("b", 40)
	runes := []rune(line)
	cursorPos := 0
	for i, r := range runes { // put the cursor on the 'T' of TARGET
		if r == 'T' {
			cursorPos = i
			break
		}
	}
	assert.Greater(t, cursorPos, 0, "sanity: found target column")

	view := NewTextWriterView(&TextWriterArgs{}).(*TextWriterView)
	view.text = line
	view.cursorPos = cursorPos
	view.updateTextLines()

	sx, sy := screen.Size()
	view.Resize(sx, sy)
	view.Draw(screen)
	screen.Show()

	// Scan the cursor row for the single reverse-video (cursor) cell.
	found := ""
	cursorFound := false
	for x := 2; x < sx-2; x++ {
		glyph, style, _ := screen.Get(x, view.textStartY)
		if style == view.cursorStyle {
			found = glyph
			cursorFound = true
			break
		}
	}
	assert.True(t, cursorFound, "cursor cell (reverse video) should be rendered")
	assert.Equal(t, "T", found, "glyph under cursor on a horizontally-scrolled multi-byte line")
}

func Test_fjiraTextWriterView_applyExternalText(t *testing.T) {
	tests := []struct {
		name        string
		initialText string
		edited      string
		wantText    string
		wantCursor  int
	}{
		{"replaces text", "old", "brand new text", "brand new text", len([]rune("brand new text"))},
		{"strips single trailing newline", "x", "line one\n", "line one", len([]rune("line one"))},
		{"keeps interior newlines", "x", "line one\nline two\n", "line one\nline two", len([]rune("line one\nline two"))},
		{"strips only one trailing newline", "x", "trailing\n\n", "trailing\n", len([]rune("trailing\n"))},
		{"multi-byte cursor is rune count", "x", "café 🚀\n", "café 🚀", len([]rune("café 🚀"))},
		// $EDITOR output is NOT length-limited (mirrors the keystroke path).
		{"does not truncate long text", "x", strings.Repeat("a", 5000), strings.Repeat("a", 5000), 5000},
		{"empty result", "something", "\n", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("utf-8")
			_ = screen.Init() //nolint:errcheck
			defer screen.Fini()
			app.InitTestApp(screen)

			view := NewTextWriterView(&TextWriterArgs{
				InitialText: tt.initialText,
			}).(*TextWriterView)

			view.applyExternalText(tt.edited)

			assert.Equal(t, tt.wantText, view.text, "text")
			assert.Equal(t, tt.wantText, view.buffer.String(), "buffer synced")
			assert.Equal(t, tt.wantCursor, view.cursorPos, "cursorPos")
			assert.Equal(t, strings.Split(tt.wantText, "\n"), view.textLines, "textLines refreshed")
		})
	}
}

func Test_fjiraTextWriterView_runExternalEditor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script editor is POSIX-only")
	}
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	// A fake $EDITOR that appends a marker to the file it's given — proves the
	// temp file is written, the arg is passed, and the result is read back.
	dir := t.TempDir()
	scriptPath := dir + "/fake-editor.sh"
	script := "#!/bin/sh\nprintf ' [edited]' >> \"$1\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write fake editor: %v", err)
	}

	view := NewTextWriterView(&TextWriterArgs{InitialText: "hello"}).(*TextWriterView)

	// Pass extra args to prove strings.Fields arg-splitting works.
	edited, ok := view.runExternalEditor("sh " + scriptPath)
	assert.True(t, ok, "round-trip should succeed")
	assert.Equal(t, "hello [edited]", edited)

	// A non-zero editor exit aborts the apply.
	failScript := dir + "/fail-editor.sh"
	if err := os.WriteFile(failScript, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write fail editor: %v", err)
	}
	_, ok = view.runExternalEditor("sh " + failScript)
	assert.False(t, ok, "non-zero editor exit should abort")
}

func Test_resolveEditor(t *testing.T) {
	origVisual, hadVisual := os.LookupEnv("VISUAL")
	origEditor, hadEditor := os.LookupEnv("EDITOR")
	restore := func() {
		if hadVisual {
			_ = os.Setenv("VISUAL", origVisual)
		} else {
			_ = os.Unsetenv("VISUAL")
		}
		if hadEditor {
			_ = os.Setenv("EDITOR", origEditor)
		} else {
			_ = os.Unsetenv("EDITOR")
		}
	}
	defer restore()

	t.Run("prefers VISUAL over EDITOR", func(t *testing.T) {
		_ = os.Setenv("VISUAL", "vim")
		_ = os.Setenv("EDITOR", "nano")
		assert.Equal(t, "vim", resolveEditor())
	})
	t.Run("falls back to EDITOR", func(t *testing.T) {
		_ = os.Unsetenv("VISUAL")
		_ = os.Setenv("EDITOR", "nano")
		assert.Equal(t, "nano", resolveEditor())
	})
	t.Run("empty when neither set", func(t *testing.T) {
		_ = os.Unsetenv("VISUAL")
		_ = os.Unsetenv("EDITOR")
		assert.Equal(t, "", resolveEditor())
	})
}

func Test_fjiraTextWriterView_InitialTextMultiByte(t *testing.T) {
	// Regression for crash on first keystroke when InitialText contains
	// multi-byte UTF-8 chars: cursorPos was initialized to byte length
	// instead of rune count, then later code indexed `runes[:cursorPos]`
	// and panicked with "slice bounds out of range".
	tests := []struct {
		name        string
		initialText string
	}{
		{"ascii only", "hello world"},
		{"em-dash (3-byte)", "before — after"},
		{"smart quotes (3-byte)", "He said “hi” to her."},
		{"emoji (4-byte)", "ship it 🚀 today"},
		{"mixed", "café — déjà 🚀 vu"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewTextWriterView(&TextWriterArgs{InitialText: tt.initialText}).(*TextWriterView)
			// cursorPos must be the rune count, not the byte length.
			assert.Equal(t, len([]rune(tt.initialText)), view.cursorPos, "cursorPos should equal rune count")
			// First keystroke previously panicked here.
			assert.NotPanics(t, func() {
				view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
			})
			// After inserting one rune, cursorPos should advance by one.
			assert.Equal(t, len([]rune(tt.initialText))+1, view.cursorPos)
		})
	}
}
