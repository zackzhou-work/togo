package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/zackzhou-work/togo/internal/store"
)

// fakeClock runs the app's timers when the test says so.
type fakeClock struct {
	now    time.Duration
	timers []fakeTimer
}

type fakeTimer struct {
	at time.Duration
	fn func()
}

func (f *fakeClock) after(d time.Duration, fn func()) {
	f.timers = append(f.timers, fakeTimer{f.now + d, fn})
}

// advance fires due timers in order, each at its own time, so a timer that
// schedules another inside the window fires that one too.
func (f *fakeClock) advance(d time.Duration) {
	end := f.now + d
	for len(f.timers) > 0 {
		i := 0
		for j, t := range f.timers {
			if t.at < f.timers[i].at {
				i = j
			}
		}
		t := f.timers[i]
		if t.at > end {
			break
		}
		f.timers = slices.Delete(f.timers, i, i+1)
		f.now = t.at
		t.fn()
	}
	f.now = end
}

type harness struct {
	t     *testing.T
	app   *app
	clock *fakeClock
	tt    *ui.Tester
}

func newHarness(t *testing.T, ongoing, inbox []string) *harness {
	t.Helper()
	db, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// InsertTask puts the newest on top, so insert bottom-up.
	for _, title := range slices.Backward(inbox) {
		if _, err := db.InsertTask(title, false, store.Inbox); err != nil {
			t.Fatal(err)
		}
	}
	for _, title := range slices.Backward(ongoing) {
		if _, err := db.InsertTask(title, false, store.Ongoing); err != nil {
			t.Fatal(err)
		}
	}
	clock := &fakeClock{}
	a := newApp(db, false, clock.after)
	return &harness{t: t, app: a, clock: clock, tt: ui.NewTester(a.view, 380, 520)}
}

func (h *harness) center(text string) (float32, float32) {
	h.t.Helper()
	r, ok := h.tt.Find(text)
	if !ok {
		h.t.Fatalf("%q not shown; texts %q", text, h.tt.Texts())
	}
	return r.X + r.W/2, r.Y + r.H/2
}

func (h *harness) hover(text string) {
	h.t.Helper()
	h.tt.Move(h.center(text))
	h.tt.Frame()
}

func (h *harness) click(label string) {
	h.t.Helper()
	if err := h.tt.Click(label); err != nil {
		h.t.Fatal(err)
	}
}

func titles(tasks []store.Task) []string {
	var out []string
	for _, t := range tasks {
		out = append(out, t.Title)
	}
	return out
}

func TestShowsBothSections(t *testing.T) {
	h := newHarness(t, []string{"写周报"}, nil)
	for _, want := range []string{"进行中", "收件箱", "写周报", "收件箱是空的", "⌘N 记一笔"} {
		if !h.tt.HasText(want) {
			t.Errorf("missing %q; texts %q", want, h.tt.Texts())
		}
	}
}

func TestNewTask(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.tt.Key(ui.Cmd, ui.KeyN)
	if !h.app.creating {
		t.Fatal("⌘N should open the field")
	}
	h.tt.Type("买牛奶")
	h.tt.Key(0, ui.KeyEnter)
	if h.app.creating {
		t.Fatal("Enter should close the field: one task per ⌘N")
	}
	if got := titles(h.app.inbox); !slices.Equal(got, []string{"买牛奶"}) {
		t.Fatalf("inbox %v", got)
	}
}

func TestNewTaskScrollsInboxToTop(t *testing.T) {
	for _, ongoing := range []int{3, 20} {
		var titles []string
		for i := range ongoing {
			titles = append(titles, "进行中的事 "+string(rune('A'+i)))
		}
		// Enough inbox below that the list can scroll the inbox to the top.
		var inbox []string
		for i := range 20 {
			inbox = append(inbox, "收件箱的事 "+string(rune('A'+i)))
		}
		h := newHarness(t, titles, inbox)
		h.tt.Key(ui.Cmd, ui.KeyN)
		h.tt.Frame()
		r, ok := h.tt.Find("收件箱")
		if !ok || r.Y > chromeHeight+sectionHeight {
			t.Errorf("%d ongoing: inbox heading at %v, want it at the top", ongoing, r)
		}
	}
}

func TestEscapeDropsNewTask(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("算了")
	h.tt.Key(0, ui.KeyEscape)
	if h.app.creating || len(h.app.inbox) != 0 {
		t.Fatalf("Escape should drop the draft; creating %v inbox %v", h.app.creating, titles(h.app.inbox))
	}
}

func TestClickingAwayKeepsNewTask(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.tt.Key(ui.Cmd, ui.KeyN)
	h.tt.Type("留着")
	h.tt.ClickAt(h.center("进行中"))
	h.tt.Frame()
	if got := titles(h.app.inbox); !slices.Equal(got, []string{"留着"}) {
		t.Fatalf("inbox %v", got)
	}
}

func TestCompletionHoldsThenCommits(t *testing.T) {
	h := newHarness(t, []string{"a", "b"}, nil)
	h.hover("a")
	h.click("完成")
	if h.app.completing[h.app.ongoing[0].ID] != held {
		t.Fatal("first click should hold the row")
	}

	h.clock.advance(completionHold)
	h.tt.Frame()
	if len(h.app.ongoing) != 2 {
		t.Fatal("nothing is written until the row has folded away")
	}
	h.clock.advance(completionCollapse)
	h.tt.Frame()
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("ongoing %v", got)
	}
	if got := titles(h.app.completed); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("completed %v", got)
	}
}

func TestSecondClickTakesCompletionBack(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.hover("a")
	h.click("完成")
	h.click("完成")
	h.clock.advance(time.Second)
	h.tt.Frame()
	if len(h.app.ongoing) != 1 || len(h.app.completed) != 0 {
		t.Fatal("a second click inside the hold should undo it")
	}
}

func TestDeleteNeedsTwoClicks(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.hover("a")
	h.click("删除")
	if len(h.app.ongoing) != 1 {
		t.Fatal("first click only arms")
	}
	h.click("删除")
	if len(h.app.ongoing) != 0 {
		t.Fatal("second click deletes")
	}
}

func TestArmedDeleteExpires(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.hover("a")
	h.click("删除")
	h.clock.advance(confirmTimeout)
	h.tt.Frame()
	h.click("删除")
	if len(h.app.ongoing) != 1 {
		t.Fatal("a click after the timeout should arm again, not delete")
	}
}

func TestCmdBackspaceDeletesHovered(t *testing.T) {
	h := newHarness(t, []string{"a", "b"}, nil)
	h.hover("b")
	h.tt.Key(ui.Cmd, ui.KeyBackspace)
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("ongoing %v", got)
	}
}

func TestRename(t *testing.T) {
	h := newHarness(t, []string{"旧标题"}, nil)
	x, y := h.center("旧标题")
	h.tt.ClickAt(x, y)
	h.tt.ClickAt(x, y)
	if h.app.editingID == "" {
		t.Fatal("double click should open the title")
	}
	h.tt.Key(ui.Cmd, ui.KeyBackspace)
	if len(h.app.ongoing) != 1 {
		t.Fatal("⌘⌫ while editing belongs to the text, not the task")
	}
	h.tt.Type("新标题")
	h.tt.Key(0, ui.KeyEnter)
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"新标题"}) {
		t.Fatalf("ongoing %v", got)
	}
}

func TestEmptyRenameKeepsOldTitle(t *testing.T) {
	h := newHarness(t, []string{"留下"}, nil)
	h.app.startEditing(h.app.ongoing[0].ID)
	h.tt.Frame()
	h.tt.Key(ui.Cmd, ui.KeyBackspace)
	h.tt.Key(0, ui.KeyEnter)
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"留下"}) {
		t.Fatalf("ongoing %v", got)
	}
}

func TestPriorityToggle(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.hover("a")
	h.click("优先")
	if !h.app.ongoing[0].IsPriority {
		t.Fatal("should be priority")
	}
}

func TestDragReorders(t *testing.T) {
	h := newHarness(t, []string{"a", "b", "c"}, nil)
	h.tt.Press(h.center("c"))
	h.tt.Move(h.center("b"))
	h.tt.Frame()
	h.tt.Release(h.center("a"))
	h.tt.Frame()
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("ongoing %v", got)
	}
}

// The marker has to follow the pointer during the drag, not only the drop at
// the end: showing it must not cost the source its identity mid-drag.
func TestDropIndicatorFollowsPointer(t *testing.T) {
	h := newHarness(t, []string{"a", "b", "c"}, nil)
	x, cy := h.center("c")
	_, ay := h.center("a")
	h.tt.Press(x, cy)
	for y := cy; y >= ay; y -= 4 {
		h.tt.Move(x, y)
		h.tt.Frame()
	}
	h.tt.Move(x, ay)
	h.tt.Frame()

	// The rule sits on the top edge of the row it would land above.
	rowTop := int(ay - rowHeight/2)
	img := h.tt.Image()
	dark := false
	for y := rowTop - 2; y <= rowTop+2; y++ {
		if r, _, _, _ := img.At(200, y).RGBA(); r>>8 < 100 {
			dark = true
		}
	}
	h.tt.Release(x, ay)
	if !dark {
		t.Fatal("no drop marker above the row under the pointer")
	}
}

// Over the row just below its own slot, a drop would change nothing, so no
// marker promises a move.
func TestNoDropIndicatorForNoOpMove(t *testing.T) {
	h := newHarness(t, []string{"a", "b", "c"}, nil)
	x, by := h.center("b")
	_, cy := h.center("c")
	h.tt.Press(x, by)
	for y := by; y <= cy; y += 4 {
		h.tt.Move(x, y)
		h.tt.Frame()
	}

	// Where the rule would sit: the top edge of c, under b's slot.
	rowTop := int(cy - rowHeight/2)
	img := h.tt.Image()
	for y := rowTop - 2; y <= rowTop+2; y++ {
		if r, _, _, _ := img.At(200, y).RGBA(); r>>8 < 100 {
			t.Fatalf("drop marker at y=%d for a drop that changes nothing", y)
		}
	}
	h.tt.Release(x, cy)
}

func TestDragAcrossSections(t *testing.T) {
	h := newHarness(t, []string{"a"}, []string{"x"})
	h.tt.Press(h.center("x"))
	h.tt.Move(h.center("进行中"))
	h.tt.Frame()
	h.tt.Release(h.center("进行中"))
	h.tt.Frame()
	if got := titles(h.app.ongoing); !slices.Equal(got, []string{"a", "x"}) {
		t.Fatalf("ongoing %v", got)
	}
}

func TestHistoryPopover(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.app.completeTask(h.app.ongoing[0].ID)
	h.clock.advance(time.Second)

	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyH)
	if !h.tt.HasText("已完成") || !h.tt.HasText("a") {
		t.Fatalf("popover should list the completed task; texts %q", h.tt.Texts())
	}

	h.click("清空已完成")
	if !h.tt.HasText("再点一次清空") || len(h.app.completed) != 1 {
		t.Fatal("first click only arms")
	}
	h.click("清空已完成")
	if len(h.app.completed) != 0 || !h.tt.HasText("勾掉的任务会留在这里") {
		t.Fatalf("second click clears; texts %q", h.tt.Texts())
	}

	h.tt.ClickAt(100, 400)
	if h.app.historyOpen {
		t.Fatal("a click on the scrim closes the popover")
	}
}

func TestRestoreFromHistory(t *testing.T) {
	h := newHarness(t, []string{"a"}, nil)
	h.app.completeTask(h.app.ongoing[0].ID)
	h.clock.advance(time.Second)
	h.app.toggleHistory()
	h.tt.Frame()
	h.hover("a")
	h.click("恢复")
	if len(h.app.ongoing) != 1 || len(h.app.completed) != 0 {
		t.Fatal("restore should put the task back")
	}
}

func TestPinShortcut(t *testing.T) {
	h := newHarness(t, nil, nil)
	var got []bool
	h.app.setPinned = func(on bool) { got = append(got, on) }
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	h.click("窗口置顶")
	if !slices.Equal(got, []bool{true, false}) {
		t.Fatalf("pin calls %v", got)
	}
}
