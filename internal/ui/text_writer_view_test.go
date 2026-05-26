package ui

import (
	"bytes"
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
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyF1, 'F', tcell.ModNone))

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
