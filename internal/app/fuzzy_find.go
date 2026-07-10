package app

import (
	"bytes"
	"fmt"
	"github.com/bep/debounce"
	"github.com/gdamore/tcell/v2"
	"github.com/sahilm/fuzzy"
	"strings"
	"time"
	"unicode"
)

type FuzzyFind struct {
	MarginTop         int
	MarginBottom      int
	Complete          chan FuzzyFindResult
	records           []string
	recordsProvider   func(query string) []string
	query             string
	fuzzyStatus       string
	title             string
	matches           fuzzy.Matches
	matchesAll        fuzzy.Matches
	buffer            bytes.Buffer
	dirty             bool
	selected          int
	screenX           int
	screenY           int
	supplierDebounce  func(f func())
	debounceDisabled  bool
	disableFuzzyMatch bool
	clearOnEsc        bool // when true, Esc with non-empty query clears it instead of completing with -1

	// rangesProvider, when set, scopes fuzzy matching and highlighting to
	// declared byte ranges of each record (see rangesProvider / MatchRange).
	// matchTargets holds the projected match strings and targetMaps maps each
	// target byte index back to its display-record byte index. All three are
	// nil for the default (whole-record) matching used by every other picker.
	rangesProvider rangesProvider
	matchTargets   []string
	targetMaps     [][]int

	boldMatchStyle   tcell.Style
	cursorStyle      tcell.Style
	highlightDefault tcell.Style
	highlightBold    tcell.Style
	boldStyle        tcell.Style
	titleStyle       tcell.Style
	defaultStyle     tcell.Style
}

type FuzzyFindResult struct {
	Index int
	Match string
}

// MatchRange is a byte range [Start, End) within a display record that fuzzy
// matching and highlighting should be restricted to. Everything outside the
// declared ranges is neither matched nor highlighted. Used by the issue
// browser so only the key and summary participate, not the type/assignee/date
// columns.
type MatchRange struct {
	Start int
	End   int
}

// rangesProvider supplies, for a query, the display records AND the matchable
// byte ranges within each record (index-aligned with the returned records).
// Returning it from a single call keeps records and ranges strictly parallel,
// so match.Index stays a valid row identity for selection.
type rangesProvider func(query string) (records []string, ranges [][]MatchRange)

// buildMatchTarget projects the matchable ranges of a display record into a
// single string used for fuzzy matching, and returns a mapping from each byte
// index in that target back to its byte index in the display record. A single
// space is inserted between ranges so adjacent regions (e.g. key and summary)
// don't fuzzy-match across the boundary; the separator maps to -1 (no display
// position).
func buildMatchTarget(display string, ranges []MatchRange) (string, []int) {
	var b strings.Builder
	mapping := make([]int, 0, len(display))
	for ri, r := range ranges {
		if ri > 0 {
			b.WriteByte(' ')
			mapping = append(mapping, -1)
		}
		if r.Start < 0 || r.End > len(display) || r.Start > r.End {
			continue
		}
		for i := r.Start; i < r.End; i++ {
			b.WriteByte(display[i])
			mapping = append(mapping, i)
		}
	}
	return b.String(), mapping
}

func (f *FuzzyFind) Destroy() {}

func (f *FuzzyFind) Init() {}

const (
	ResultsMarginBottom     = 3
	WriteIndicator          = "> "
	MaxResults              = 4096
	DefaultSupplierDebounce = 50 * time.Millisecond
	SearchResultsPivot      = 6
)

func NewFuzzyFind(title string, records []string) *FuzzyFind {
	matchesAll := make(fuzzy.Matches, 0, MaxResults)
	// TODO - not super optimize way to store results..
	for i, record := range records {
		matchesAll = append(matchesAll, fuzzy.Match{
			Str:   record,
			Index: i,
		})
	}
	highlightDefaultStyle := DefaultStyle().Foreground(Color("finder.highlight.foreground")).Background(Color("finder.highlight.background"))
	return &FuzzyFind{
		Complete:          make(chan FuzzyFindResult),
		records:           records,
		query:             EmptyLine,
		fuzzyStatus:       "0/0",
		matches:           nil,
		selected:          0,
		dirty:             true,
		matchesAll:        matchesAll,
		recordsProvider:   nil,
		title:             title,
		MarginTop:         0,
		MarginBottom:      1,
		debounceDisabled:  false,
		disableFuzzyMatch: false,

		boldMatchStyle:   DefaultStyle().Foreground(Color("finder.match")).Underline(true).Bold(true),
		cursorStyle:      DefaultStyle().Foreground(Color("finder.cursor")).Bold(true),
		highlightDefault: highlightDefaultStyle,
		highlightBold:    highlightDefaultStyle.Foreground(Color("finder.highlight.match")).Bold(true),
		boldStyle:        DefaultStyle().Bold(true),
		titleStyle:       DefaultStyle().Italic(true).Foreground(Color("finder.title")),
		defaultStyle:     DefaultStyle(),
	}
}

func NewFuzzyFindWithProvider(title string, recordsProvider func(query string) []string) *FuzzyFind {
	highlightDefaultStyle := DefaultStyle().Foreground(Color("finder.highlight.foreground")).Background(Color("finder.highlight.background"))
	return &FuzzyFind{
		Complete:          make(chan FuzzyFindResult),
		records:           nil,
		query:             "init",
		fuzzyStatus:       "0/0",
		matches:           nil,
		selected:          0,
		dirty:             true,
		matchesAll:        make(fuzzy.Matches, 0, MaxResults),
		recordsProvider:   recordsProvider,
		supplierDebounce:  debounce.New(DefaultSupplierDebounce),
		title:             title,
		MarginTop:         0,
		MarginBottom:      1,
		debounceDisabled:  false,
		disableFuzzyMatch: false,

		boldMatchStyle:   DefaultStyle().Foreground(Color("finder.match")).Underline(true).Bold(true),
		cursorStyle:      DefaultStyle().Foreground(Color("finder.cursor")).Bold(true),
		highlightDefault: highlightDefaultStyle,
		highlightBold:    highlightDefaultStyle.Foreground(Color("finder.highlight.match")).Bold(true),
		boldStyle:        DefaultStyle().Bold(true),
		titleStyle:       DefaultStyle().Italic(true).Foreground(Color("finder.title")),
		defaultStyle:     DefaultStyle(),
	}
}

// NewFuzzyFindWithRangeProvider is like NewFuzzyFindWithProvider but the
// provider also returns, per record, the byte ranges that fuzzy matching and
// highlighting are restricted to. Records and ranges come from one call so they
// stay strictly parallel and match.Index remains a valid row identity.
func NewFuzzyFindWithRangeProvider(title string, provider rangesProvider) *FuzzyFind {
	f := NewFuzzyFindWithProvider(title, nil)
	f.rangesProvider = provider
	return f
}

func (f *FuzzyFind) Draw(screen tcell.Screen) {
	if f.screenX == 0 || f.screenY == 0 {
		x, y := screen.Size()
		f.screenX = x
		f.screenY = y
	}
	f.drawRecords(screen)
	if f.title != "" {
		DrawText(screen, 2, f.screenY-ResultsMarginBottom-f.MarginBottom+1, f.titleStyle, f.title)
	}
	DrawText(screen, f.screenX-len(f.fuzzyStatus)-2, f.screenY-ResultsMarginBottom-f.MarginBottom+1, f.titleStyle, f.fuzzyStatus)
	DrawText(screen, 0, f.screenY-1-f.MarginBottom, f.boldStyle, WriteIndicator)
	DrawText(screen, 2, f.screenY-1-f.MarginBottom, f.defaultStyle, f.query)
	screen.ShowCursor(2+len(f.query), f.screenY-1-f.MarginBottom)
}

func (f *FuzzyFind) Update() {
	if !f.dirty {
		return
	}
	buff := f.buffer.String()
	if (f.recordsProvider != nil || f.rangesProvider != nil) && (f.query != buff) {
		f.query = buff
		if f.debounceDisabled {
			f.updateRecordsFromSupplier()
		} else {
			f.supplierDebounce(f.updateRecordsFromSupplier)
			f.dirty = false
			return
		}
	}
	f.query = buff
	if len(f.query) == 0 || f.disableFuzzyMatch {
		f.matches = f.matchesAll
	} else if f.matchTargets != nil {
		// Match against the projected targets (key+summary only), then remap
		// each matched index back to its position in the display record so
		// highlighting lands on the right characters and never on the masked
		// columns. match.Index stays the row index (targets are parallel).
		f.matches = fuzzy.Find(f.query, f.matchTargets)
		for mi := range f.matches {
			f.matches[mi] = f.remapMatchToDisplay(f.matches[mi])
		}
	} else {
		f.matches = fuzzy.Find(f.query, f.records)
	}
	f.fuzzyStatus = fmt.Sprintf("%d/%d", len(f.matches), len(f.records))
	f.selected = ClampInt(f.selected, 0, f.matches.Len()-1)
	f.dirty = false
}

func (f *FuzzyFind) ForceUpdate() {
	f.markAsDirty()
	f.Update()
}

// Matches returns the current match set (after scoring/remapping). Exposed for
// tests that assert highlight ranges and selection identity.
func (f *FuzzyFind) Matches() fuzzy.Matches {
	return f.matches
}

func (f *FuzzyFind) HandleKeyEvent(ev *tcell.EventKey) {
	if ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyEscape {
		// SetClearOnEsc(true) opt-in: first Esc with a non-empty query just
		// clears the query (lets the user start over without leaving the
		// view). A second Esc on an empty query still completes with -1.
		// Ctrl-C always completes regardless — emergency-exit semantics.
		if f.clearOnEsc && ev.Key() == tcell.KeyEscape && f.buffer.Len() > 0 {
			f.buffer.Reset()
			f.markAsDirty()
		} else {
			f.Complete <- FuzzyFindResult{Index: -1, Match: ""}
		}
	}
	if ev.Key() == tcell.KeyEnter {
		f.markAsDirty()
		f.Update()
		if len(f.matches) > 0 && f.selected >= 0 {
			match := f.matches[f.selected].Str
			index := findSelectedRecord(match, f.records)
			f.Complete <- FuzzyFindResult{Index: index, Match: match}
		} else {
			f.Complete <- FuzzyFindResult{Index: -1, Match: ""}
		}
	}
	if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
		if f.buffer.Len() > 0 {
			f.buffer.Truncate(f.buffer.Len() - 1)
		}
		f.markAsDirty()
	}
	if ev.Key() == tcell.KeyUp || ev.Key() == tcell.KeyTab {
		f.selected = ClampInt(f.selected+1, 0, f.matches.Len()-1)
		return
	}
	if ev.Key() == tcell.KeyDown || ev.Key() == tcell.KeyBacktab {
		f.selected = ClampInt(f.selected-1, 0, f.matches.Len()-1)
		return
	}
	if ev.Key() == tcell.KeyPgUp {
		f.selected = ClampInt(f.selected+10, 0, f.matches.Len()-1)
		return
	}
	if ev.Key() == tcell.KeyPgDn {
		f.selected = ClampInt(f.selected-10, 0, f.matches.Len()-1)
		return
	}
	if f.isEventWritable(ev) {
		f.buffer.WriteRune(ev.Rune())
		f.markAsDirty()
	}
}

func (f *FuzzyFind) Resize(screenX, screenY int) {
	f.screenX = screenX
	f.screenY = screenY
}

func (f *FuzzyFind) GetQuery() string {
	return f.query
}

func (f *FuzzyFind) SetQuery(q string) {
	f.buffer.WriteString(q)
	f.markAsDirty()
}

func (f *FuzzyFind) AlwaysShowAllResults() {
	// it's a bit weird feature ... to disable fuzzy match in fuzzy finder
	// maybe part of the logic should be extracted from that fuzzy finder
	// and then simple "records table" could be displayed without fuzzy-find functionality, and
	// without such a weird stuff
	f.disableFuzzyMatch = true
}

func (f *FuzzyFind) GetSelectedItem() string {
	if len(f.records) == 0 {
		return ""
	}
	return f.records[f.selected]
}

func (f *FuzzyFind) SetDebounceDisabled(b bool) {
	f.debounceDisabled = b
}

// SetClearOnEsc opts in to "Esc clears non-empty query, second Esc exits"
// semantics. Default (false) keeps the immediate-exit behavior used by
// project/workspace pickers where Esc should always back out. Ctrl-C
// always exits regardless of this flag.
func (f *FuzzyFind) SetClearOnEsc(b bool) {
	f.clearOnEsc = b
}

func (f *FuzzyFind) SetDebounceMs(d time.Duration) {
	f.supplierDebounce = debounce.New(d)
}

// remapMatchToDisplay converts a match computed against a projected target back
// to the display record: Str becomes the display record and MatchedIndexes are
// translated from target byte offsets to display byte offsets (dropping the
// separator positions, which map to -1). match.Index (the row) is unchanged.
func (f *FuzzyFind) remapMatchToDisplay(m fuzzy.Match) fuzzy.Match {
	if m.Index < 0 || m.Index >= len(f.records) {
		return m
	}
	mapping := f.targetMaps[m.Index]
	remapped := make([]int, 0, len(m.MatchedIndexes))
	for _, ti := range m.MatchedIndexes {
		if ti >= 0 && ti < len(mapping) && mapping[ti] >= 0 {
			remapped = append(remapped, mapping[ti])
		}
	}
	m.Str = f.records[m.Index]
	m.MatchedIndexes = remapped
	return m
}

func (f *FuzzyFind) drawRecords(screen tcell.Screen) {
	matchesLen := f.matches.Len()
	if matchesLen == 0 {
		return
	}
	var row = f.screenY - ResultsMarginBottom - f.MarginBottom
	var currentStyleDefault tcell.Style
	var currentStyleBold tcell.Style
	indexDelta := ClampInt(f.selected-row+SearchResultsPivot, 0, matchesLen-1)
	for index := indexDelta; index < matchesLen && row > f.MarginTop; index++ {
		match := f.matches[index]
		currentStyleDefault = f.defaultStyle
		currentStyleBold = f.boldMatchStyle
		if index == f.selected {
			DrawText(screen, 0, row, f.cursorStyle, WriteIndicator)
			currentStyleDefault = f.highlightDefault
			currentStyleBold = f.highlightBold
		}
		runeI := 0
		for i, s := range match.Str {
			if contains(i, match.MatchedIndexes) {
				DrawText(screen, runeI+2, row, currentStyleBold, string(s))
			} else {
				DrawText(screen, runeI+2, row, currentStyleDefault, string(s))
			}
			runeI++
		}
		row--
	}
}

func (f *FuzzyFind) updateRecordsFromSupplier() {
	if f.rangesProvider != nil {
		records, ranges := f.rangesProvider(f.query)
		f.records = records
		f.matchTargets = make([]string, len(records))
		f.targetMaps = make([][]int, len(records))
		for i, record := range records {
			var rr []MatchRange
			if i < len(ranges) {
				rr = ranges[i]
			}
			f.matchTargets[i], f.targetMaps[i] = buildMatchTarget(record, rr)
		}
	} else {
		f.records = f.recordsProvider(f.query)
		f.matchTargets = nil
		f.targetMaps = nil
	}
	f.matchesAll = nil
	for i, record := range f.records {
		f.matchesAll = append(f.matchesAll, fuzzy.Match{
			Str:   record,
			Index: i,
		})
	}
	f.markAsDirty()
}

func (f *FuzzyFind) isEventWritable(ev *tcell.EventKey) bool {
	return unicode.IsLetter(ev.Rune()) || unicode.IsSpace(ev.Rune()) || unicode.IsDigit(ev.Rune()) ||
		ev.Rune() == '-' || ev.Rune() == '"' || ev.Rune() == '\'' || ev.Rune() == '&' ||
		ev.Rune() == ';' || ev.Rune() == '|' || ev.Rune() == '>' || ev.Rune() == '<' || ev.Rune() == '=' ||
		ev.Rune() == '!' || ev.Rune() == '.'
}

func (f *FuzzyFind) markAsDirty() {
	f.dirty = true
	// it couples fuzzyFinder with app ... which is not nice,
	// but it's the easy way to make sure that app is re-rendered
	// whenever fuzzy search is updated. Another solution would be to
	// expose dirty channel from here... and handle it from every place
	// which is using fuzzy finder. Let's stick to that one for a time being...
	GetApp().RunOnAppRoutine(func() {
		GetApp().SetDirty()
	})
}

func contains(needle int, haystack []int) bool {
	for _, i := range haystack {
		if needle == i {
			return true
		}
	}
	return false
}

func findSelectedRecord(result string, records []string) int {
	// TODO - impl faster alg
	var index int
	for i := range records {
		if strings.TrimSpace(records[i]) == result || records[i] == result {
			index = i
			break
		}
	}
	return index
}
