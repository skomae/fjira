package ui

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
)

// TODO - should be here?
type TextWriterView struct {
	app.View
	bottomBar   *app.ActionBar
	buffer      bytes.Buffer
	text        string
	cursorPos   int      // Current cursor position in the text
	scrollY     int      // Vertical scroll position
	textLines   []string // Text split into lines for rendering
	desiredCol  int      // Desired column position for up/down movement
	headerStyle tcell.Style
	style       tcell.Style
	cursorStyle tcell.Style
	args        TextWriterArgs
	screenX     int
	screenY     int
	textStartY  int // Y position where text area starts
}

type TextWriterArgs struct {
	MaxLength    int
	TextConsumer func(string)
	GoBack       func()
	Header       string
	InitialText  string
}

func NewTextWriterView(args *TextWriterArgs) app.View {
	bottomBar := CreateBottomLeftBar()
	bottomBar.AddItem(NewSaveBarItem())
	bottomBar.AddItem(NewCancelBarItem())
	if args.MaxLength == 0 {
		args.MaxLength = 150
	}
	if args.GoBack == nil {
		args.GoBack = func() {
		}
	}
	if args.TextConsumer == nil {
		args.TextConsumer = func(str string) {
		}
	}

	view := &TextWriterView{
		bottomBar:   bottomBar,
		text:        args.InitialText,
		cursorPos:   len(args.InitialText), // Start cursor at end of initial text
		scrollY:     0,
		textStartY:  4, // Header at y=2, text starts at y=4
		desiredCol:  0, // Will be updated when cursor moves
		args:        *args,
		headerStyle: app.DefaultStyle().Foreground(app.Color("default.foreground2")).Underline(true),
		style:       app.DefaultStyle(),
		cursorStyle: app.DefaultStyle().Reverse(true), // Cursor with reverse video
	}

	// Initialize buffer with initial text if provided
	if args.InitialText != "" {
		view.buffer.WriteString(args.InitialText)
	}

	view.updateTextLines()
	// Initialize desired column based on initial cursor position
	_, col := view.getCursorLineCol()
	view.desiredCol = col
	return view
}

// updateTextLines splits the text into lines for rendering
func (view *TextWriterView) updateTextLines() {
	view.textLines = strings.Split(view.text, "\n")
}

// getCursorLineCol returns the line and column position of the cursor
func (view *TextWriterView) getCursorLineCol() (int, int) {
	line := 0
	col := 0
	pos := 0

	for _, r := range view.text {
		if pos == view.cursorPos {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		pos++
	}
	return line, col
}

func (view *TextWriterView) Init() {
	go view.handleBottomBarActions()
}

func (view *TextWriterView) Destroy() {
	view.bottomBar.Destroy()
}

func (view *TextWriterView) Draw(screen tcell.Screen) {
	// Draw header
	app.DrawText(screen, 1, 2, view.headerStyle, view.args.Header)

	// Calculate text area dimensions - account for gutters
	textWidth := view.screenX - 4                    // Leave margin (1) + gutters (1) on both sides
	textHeight := view.screenY - view.textStartY - 3 // Leave space for bottom bar

	if textWidth <= 0 || textHeight <= 0 {
		view.bottomBar.Draw(screen)
		return
	}

	// Get cursor position
	cursorLine, cursorCol := view.getCursorLineCol()

	// Render visible lines
	for i := 0; i < textHeight && i+view.scrollY < len(view.textLines); i++ {
		lineIdx := i + view.scrollY
		line := view.textLines[lineIdx]
		screenY := view.textStartY + i

		// Determine how to display this line
		var displayText string
		var leftTruncated, rightTruncated bool
		leftGutter := ' '
		rightGutter := ' '

		if lineIdx == cursorLine {
			// For cursor line, scroll horizontally to keep cursor visible
			displayText, leftTruncated, rightTruncated = view.getScrolledLineForCursor(line, cursorCol, textWidth)
		} else {
			// For non-cursor lines, simple truncation
			lineRunes := []rune(line)
			if len(lineRunes) <= textWidth {
				displayText = line
			} else {
				displayText = string(lineRunes[:textWidth])
				rightTruncated = true
			}
		}

		// Set gutter indicators
		if leftTruncated {
			leftGutter = '$'
		}
		if rightTruncated {
			rightGutter = '$'
		}

		// Draw gutters
		screen.SetContent(1, screenY, leftGutter, nil, view.style)
		screen.SetContent(view.screenX-2, screenY, rightGutter, nil, view.style)

		// Draw text content
		if displayText != "" {
			app.DrawText(screen, 2, screenY, view.style, displayText)
		}

		// Draw cursor if it's on this line and visible
		if lineIdx == cursorLine {
			cursorX := view.getCursorScreenX(cursorCol, displayText, leftTruncated, textWidth)
			if cursorX >= 2 && cursorX < view.screenX-2 {
				cursorChar := ' '
				if cursorX-2 < len(displayText) {
					cursorChar = rune(displayText[cursorX-2])
				}
				screen.SetContent(cursorX, screenY, cursorChar, nil, view.cursorStyle)
			}
		}
	}

	view.bottomBar.Draw(screen)
}

// getScrolledLineForCursor returns the line text scrolled to keep cursor visible with 4-char margin
func (view *TextWriterView) getScrolledLineForCursor(line string, cursorCol, textWidth int) (string, bool, bool) {
	runes := []rune(line)
	if len(runes) <= textWidth {
		return line, false, false
	}

	margin := 4

	// Calculate ideal scroll position to center cursor
	idealStart := cursorCol - textWidth/2

	// Ensure cursor has at least margin chars from edges
	minStart := cursorCol - textWidth + margin + 1
	maxStart := cursorCol - margin

	// Clamp to valid range
	start := idealStart
	if start < 0 {
		start = 0
	}
	if start > len(runes)-textWidth {
		start = len(runes) - textWidth
	}
	if start < minStart {
		start = minStart
	}
	if start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}

	end := start + textWidth
	if end > len(runes) {
		end = len(runes)
	}

	displayText := string(runes[start:end])
	leftTruncated := start > 0
	rightTruncated := end < len(runes)

	return displayText, leftTruncated, rightTruncated
}

// getCursorScreenX returns the screen X position for the cursor
func (view *TextWriterView) getCursorScreenX(cursorCol int, displayText string, leftTruncated bool, textWidth int) int {
	if leftTruncated {
		// Calculate how much we scrolled from the left
		originalLine := view.textLines[view.getCursorLine()]
		originalRunes := []rune(originalLine)
		displayRunes := []rune(displayText)

		for i := 0; i <= len(originalRunes)-len(displayRunes); i++ {
			if string(originalRunes[i:i+len(displayRunes)]) == displayText {
				scrollOffset := i
				return 2 + (cursorCol - scrollOffset)
			}
		}
	}
	return 2 + cursorCol
}

// getCursorLine returns just the line number of the cursor
func (view *TextWriterView) getCursorLine() int {
	line, _ := view.getCursorLineCol()
	return line
}

func (view *TextWriterView) Update() {
	view.bottomBar.Update()
}

func (view *TextWriterView) Resize(screenX, screenY int) {
	view.screenX = screenX
	view.screenY = screenY
	view.bottomBar.Resize(screenX, screenY)
}

func (view *TextWriterView) HandleKeyEvent(ev *tcell.EventKey) {
	view.bottomBar.HandleKeyEvent(ev)

	oldText := view.text

	switch ev.Key() {
	case tcell.KeyLeft:
		if ev.Modifiers()&tcell.ModShift != 0 {
			view.moveCursorLeftBy(5)
		} else {
			view.moveCursorLeftBy(1)
		}
	case tcell.KeyRight:
		if ev.Modifiers()&tcell.ModShift != 0 {
			view.moveCursorRightBy(5)
		} else {
			view.moveCursorRightBy(1)
		}
	case tcell.KeyUp:
		if ev.Modifiers()&tcell.ModShift != 0 {
			view.moveCursorUpBy(3)
		} else {
			view.moveCursorUpBy(1)
		}
	case tcell.KeyDown:
		if ev.Modifiers()&tcell.ModShift != 0 {
			view.moveCursorDownBy(3)
		} else {
			view.moveCursorDownBy(1)
		}
	case tcell.KeyHome:
		view.moveCursorToLineStart()
		view.updateDesiredCol()
	case tcell.KeyEnd:
		view.moveCursorToLineEnd()
		view.updateDesiredCol()
	case tcell.KeyEnter:
		// Insert newline at cursor position
		view.insertTextAtCursor("\n")
		view.updateDesiredCol()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if view.cursorPos > 0 {
			// Remove character before cursor
			runes := []rune(view.text)
			if view.cursorPos <= len(runes) {
				newRunes := append(runes[:view.cursorPos-1], runes[view.cursorPos:]...)
				view.text = string(newRunes)
				view.cursorPos--
				view.syncBuffer()
				view.updateDesiredCol()
			}
		}
	case tcell.KeyDelete:
		if view.cursorPos < len(view.text) {
			// Remove character at cursor position
			runes := []rune(view.text)
			if view.cursorPos < len(runes) {
				newRunes := append(runes[:view.cursorPos], runes[view.cursorPos+1:]...)
				view.text = string(newRunes)
				view.syncBuffer()
				view.updateDesiredCol()
			}
		}
	default:
		// Handle regular character input
		if (unicode.IsLetter(ev.Rune()) || unicode.IsDigit(ev.Rune()) || unicode.IsSpace(ev.Rune()) ||
			unicode.IsPunct(ev.Rune()) || unicode.IsSymbol(ev.Rune())) && ev.Rune() != 0 {
			view.insertTextAtCursor(string(ev.Rune()))
			view.updateDesiredCol()
		}
	}

	// Update text lines if text changed
	if view.text != oldText {
		view.updateTextLines()
	}

	// Ensure cursor is visible
	view.ensureCursorVisible()
}

// insertTextAtCursor inserts text at the current cursor position
func (view *TextWriterView) insertTextAtCursor(text string) {
	runes := []rune(view.text)
	newRunes := append(runes[:view.cursorPos], append([]rune(text), runes[view.cursorPos:]...)...)
	view.text = string(newRunes)
	view.cursorPos += len([]rune(text))
	view.syncBuffer()
}

// syncBuffer updates the buffer to match the current text
func (view *TextWriterView) syncBuffer() {
	view.buffer.Reset()
	view.buffer.WriteString(view.text)
}

// updateDesiredCol updates the desired column based on current cursor position
func (view *TextWriterView) updateDesiredCol() {
	_, col := view.getCursorLineCol()
	view.desiredCol = col
}

// moveCursorLeftBy moves cursor left by the specified number of characters
func (view *TextWriterView) moveCursorLeftBy(steps int) {
	newPos := view.cursorPos - steps
	if newPos < 0 {
		newPos = 0
	}
	view.cursorPos = newPos
	view.updateDesiredCol()
}

// moveCursorRightBy moves cursor right by the specified number of characters
func (view *TextWriterView) moveCursorRightBy(steps int) {
	runes := []rune(view.text)
	newPos := view.cursorPos + steps
	if newPos > len(runes) {
		newPos = len(runes)
	}
	view.cursorPos = newPos
	view.updateDesiredCol()
}

// moveCursorUpBy moves cursor up by the specified number of lines
func (view *TextWriterView) moveCursorUpBy(steps int) {
	cursorLine, _ := view.getCursorLineCol()
	targetLine := cursorLine - steps
	if targetLine < 0 {
		targetLine = 0
	}

	if targetLine < len(view.textLines) {
		targetLineLength := len([]rune(view.textLines[targetLine]))
		newCol := view.desiredCol
		if newCol > targetLineLength {
			newCol = targetLineLength
		}
		view.setCursorToLineCol(targetLine, newCol)
	}
}

// moveCursorDownBy moves cursor down by the specified number of lines
func (view *TextWriterView) moveCursorDownBy(steps int) {
	cursorLine, _ := view.getCursorLineCol()
	targetLine := cursorLine + steps
	if targetLine >= len(view.textLines) {
		targetLine = len(view.textLines) - 1
	}

	if targetLine >= 0 && targetLine < len(view.textLines) {
		targetLineLength := len([]rune(view.textLines[targetLine]))
		newCol := view.desiredCol
		if newCol > targetLineLength {
			newCol = targetLineLength
		}
		view.setCursorToLineCol(targetLine, newCol)
	}
}

// moveCursorUp moves cursor to the line above, using desired column
func (view *TextWriterView) moveCursorUp() {
	view.moveCursorUpBy(1)
}

// moveCursorDown moves cursor to the line below, using desired column
func (view *TextWriterView) moveCursorDown() {
	view.moveCursorDownBy(1)
}

// moveCursorToLineStart moves cursor to the beginning of current line
func (view *TextWriterView) moveCursorToLineStart() {
	cursorLine, _ := view.getCursorLineCol()
	view.setCursorToLineCol(cursorLine, 0)
}

// moveCursorToLineEnd moves cursor to the end of current line
func (view *TextWriterView) moveCursorToLineEnd() {
	cursorLine, _ := view.getCursorLineCol()
	lineLength := len([]rune(view.textLines[cursorLine]))
	view.setCursorToLineCol(cursorLine, lineLength)
}

// setCursorToLineCol sets cursor to specific line and column (rune-based)
func (view *TextWriterView) setCursorToLineCol(line, col int) {
	runes := []rune(view.text)
	pos := 0
	currentLine := 0

	// Find the start of the target line
	for i, r := range runes {
		if currentLine == line {
			break
		}
		if r == '\n' {
			currentLine++
		}
		pos = i + 1
	}

	// Add the column offset within the line
	runesInTargetLine := 0
	for i := pos; i < len(runes) && runes[i] != '\n'; i++ {
		if runesInTargetLine == col {
			break
		}
		runesInTargetLine++
		pos++
	}

	// Clamp to valid range
	if pos > len(runes) {
		pos = len(runes)
	}
	if pos < 0 {
		pos = 0
	}

	view.cursorPos = pos
}

// ensureCursorVisible adjusts vertical scrolling to ensure cursor line is visible
func (view *TextWriterView) ensureCursorVisible() {
	if view.screenX <= 0 || view.screenY <= 0 {
		return
	}

	cursorLine := view.getCursorLine()
	textHeight := view.screenY - view.textStartY - 3

	// Vertical scrolling only
	if cursorLine < view.scrollY {
		view.scrollY = cursorLine
	} else if cursorLine >= view.scrollY+textHeight {
		view.scrollY = cursorLine - textHeight + 1
	}

	if view.scrollY < 0 {
		view.scrollY = 0
	}
}

func (view *TextWriterView) handleBottomBarActions() {
	action := <-view.bottomBar.Action
	switch action {
	case ActionYes:
		view.args.TextConsumer(view.buffer.String())
	}
	go view.args.GoBack()
}
