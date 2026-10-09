package main

import (
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/zackzhou-work/togo/internal/store"
)

const (
	// Long enough to read the strike-through, and the whole window in which a
	// mis-click can be taken back.
	completionHold = 240 * time.Millisecond
	// The row folding shut afterwards, which commits the write.
	completionCollapse = 160 * time.Millisecond
	// An armed destructive button forgets it was armed after this, so a click
	// landing minutes later cannot finish something started by accident.
	confirmTimeout = 3 * time.Second
)

// completing is where a checked row is in its two-step exit.
type completing int

const (
	// Struck through and holding still. Checking it again puts it back.
	held completing = iota + 1
	// Folding out of the list. Past the point of return.
	leaving
)

type app struct {
	db *store.Store
	// after runs fn on the UI thread once d has passed. Tests swap in a clock
	// they advance by hand.
	after func(d time.Duration, fn func())
	// setPinned reaches the window and the prefs file; nil in tests.
	setPinned func(bool)

	inbox, ongoing, completed []store.Task

	pinned      bool
	historyOpen bool

	creating bool
	draft    string
	// Set by ⌘N and consumed by the next frame, which scrolls the inbox to
	// the top of the window.
	revealDraft bool
	scroll      ui.ScrollState

	// Renaming has its own buffer: an unsaved new task and an edit to a saved
	// one end differently.
	editingID string
	editDraft string

	// The row under the pointer, rebuilt every frame. Only ⌘⌫ reads it.
	hoveredID     string
	confirmDelete string
	confirmClear  bool
	completing    map[string]completing
}

func newApp(db *store.Store, pinned bool, after func(time.Duration, func())) *app {
	a := &app{
		db:         db,
		pinned:     pinned,
		after:      after,
		completing: map[string]completing{},
	}
	a.refresh()
	return a
}

func (a *app) refresh() {
	if tasks, err := a.db.ActiveTasks(store.Inbox); err == nil {
		a.inbox = tasks
	}
	if tasks, err := a.db.ActiveTasks(store.Ongoing); err == nil {
		a.ongoing = tasks
	}
	if tasks, err := a.db.CompletedTasks(); err == nil {
		a.completed = tasks
	}
}

func (a *app) check(err error) {
	if err != nil {
		log.Printf("[db] %v", err)
	}
}

func (a *app) isEditing() bool { return a.creating || a.editingID != "" }

func (a *app) togglePin() {
	a.pinned = !a.pinned
	if a.setPinned != nil {
		a.setPinned(a.pinned)
	}
}

// toggleHistory finishes whatever was half-written first: a new task is
// dropped, a rename is kept.
func (a *app) toggleHistory() {
	a.commitEdit()
	a.creating = false
	a.historyOpen = !a.historyOpen
	a.confirmClear = false
	a.confirmDelete = ""
	if a.historyOpen {
		a.refresh()
	}
}

func (a *app) startCreating() {
	// Re-entering while the field is open would wipe what is in it.
	if a.creating {
		return
	}
	a.commitEdit()
	a.creating = true
	a.historyOpen = false
	a.draft = ""
	a.revealDraft = true
}

// commitNewTask saves one task per ⌘N. Keeping the field open for a second
// thought costs more often than it helps.
func (a *app) commitNewTask() {
	if title := strings.TrimSpace(a.draft); title != "" {
		_, err := a.db.InsertTask(title, false, store.Inbox)
		a.check(err)
		a.refresh()
	}
	a.draft = ""
	a.creating = false
}

func (a *app) cancelNewTask() {
	a.draft = ""
	a.creating = false
}

func (a *app) startEditing(id string) {
	if a.editingID == id {
		return
	}
	a.commitEdit()
	a.creating = false
	title, ok := a.findTitle(id)
	if !ok {
		return
	}
	a.historyOpen = false
	a.confirmDelete = ""
	a.editingID = id
	a.editDraft = title
}

// commitEdit keeps the new title unless it is empty: emptying a title is not
// a way to delete.
func (a *app) commitEdit() {
	if a.editingID == "" {
		return
	}
	if title := strings.TrimSpace(a.editDraft); title != "" {
		a.check(a.db.UpdateTitle(a.editingID, title))
		a.refresh()
	}
	a.editingID = ""
}

func (a *app) cancelEdit() { a.editingID = "" }

func (a *app) cancel() {
	switch {
	case a.editingID != "":
		a.cancelEdit()
	case a.creating:
		a.cancelNewTask()
	case a.historyOpen:
		a.toggleHistory()
	}
}

func (a *app) findTitle(id string) (string, bool) {
	for _, list := range [][]store.Task{a.ongoing, a.inbox} {
		for _, t := range list {
			if t.ID == id {
				return t.Title, true
			}
		}
	}
	return "", false
}

// completeTask strikes the row through, holds it long enough to be taken
// back, then folds it away and writes it.
func (a *app) completeTask(id string) {
	switch a.completing[id] {
	case held:
		delete(a.completing, id)
		return
	case leaving:
		return
	}
	a.completing[id] = held
	a.after(completionHold, func() {
		if a.completing[id] != held {
			return
		}
		a.completing[id] = leaving
		a.after(completionCollapse, func() {
			delete(a.completing, id)
			a.check(a.db.CompleteTask(id))
			a.refresh()
		})
	})
}

func (a *app) restoreTask(id string) {
	a.check(a.db.RestoreTask(id))
	a.refresh()
}

// requestDelete arms on the first click and deletes on the second. ⌘⌫ skips
// the arming step: reaching for it is already deliberate in a way brushing a
// 21px button is not.
func (a *app) requestDelete(id string) {
	if a.confirmDelete == id {
		a.confirmDelete = ""
		a.deleteTask(id)
		return
	}
	a.confirmDelete = id
	a.after(confirmTimeout, func() {
		if a.confirmDelete == id {
			a.confirmDelete = ""
		}
	})
}

func (a *app) deleteTask(id string) {
	a.check(a.db.DeleteTask(id))
	if a.hoveredID == id {
		a.hoveredID = ""
	}
	a.refresh()
}

func (a *app) deleteHovered() {
	if a.hoveredID != "" && !a.isEditing() {
		a.deleteTask(a.hoveredID)
	}
}

func (a *app) reposition(id string, column store.Column, before string) {
	a.check(a.db.RepositionTask(id, column, before))
	a.refresh()
}

func (a *app) togglePriority(id string) {
	_, err := a.db.TogglePriority(id)
	a.check(err)
	a.refresh()
}

func (a *app) clearCompleted() {
	if !a.confirmClear {
		a.confirmClear = true
		a.after(confirmTimeout, func() { a.confirmClear = false })
		return
	}
	a.check(a.db.ClearCompleted())
	a.confirmClear = false
	a.refresh()
}
