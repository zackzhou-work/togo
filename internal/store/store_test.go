package store

import (
	"slices"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func titles(t *testing.T, s *Store, column Column) []string {
	t.Helper()
	var out []string
	for _, task := range must[[]Task](t)(s.ActiveTasks(column)) {
		out = append(out, task.Title)
	}
	return out
}

func TestTaskLifecycle(t *testing.T) {
	s := newTestStore(t)
	insert := must[Task](t)

	task1 := insert(s.InsertTask("normal", false, Inbox))
	task2 := insert(s.InsertTask("priority", true, Inbox))
	if task1.Column != Inbox || task1.IsPriority || !task2.IsPriority {
		t.Fatalf("unexpected inserted tasks: %+v %+v", task1, task2)
	}

	if got := titles(t, s, Inbox); !slices.Equal(got, []string{"priority", "normal"}) {
		t.Fatalf("newest should sit on top, got %v", got)
	}

	ok(t, s.RepositionTask(task2.ID, Inbox, ""))
	if got := titles(t, s, Inbox); !slices.Equal(got, []string{"normal", "priority"}) {
		t.Fatalf("hand ordering should win over priority, got %v", got)
	}

	ok(t, s.MoveTaskColumn(task1.ID, Ongoing))
	if len(titles(t, s, Inbox)) != 1 || len(titles(t, s, Ongoing)) != 1 {
		t.Fatal("move should take the task out of Inbox into Ongoing")
	}

	ok(t, s.CompleteTask(task1.ID))
	ok(t, s.CompleteTask(task2.ID))
	completed := must[[]Task](t)(s.CompletedTasks())
	if len(completed) != 2 || len(titles(t, s, Inbox))+len(titles(t, s, Ongoing)) != 0 {
		t.Fatalf("both tasks should be completed, got %d", len(completed))
	}
	if completed[0].CompletedAt == nil {
		t.Fatal("completed task should carry a completion time")
	}

	ok(t, s.RestoreTask(task1.ID))
	if len(must[[]Task](t)(s.CompletedTasks())) != 1 || len(titles(t, s, Ongoing)) != 1 {
		t.Fatal("restore should put the task back in its column")
	}

	ok(t, s.ClearCompleted())
	if len(must[[]Task](t)(s.CompletedTasks())) != 0 {
		t.Fatal("clear should remove every completed task")
	}
}

func TestRepositionDownwardLandsAboveAnchor(t *testing.T) {
	s := newTestStore(t)
	insert := must[Task](t)
	a := insert(s.InsertTask("a", false, Ongoing))
	insert(s.InsertTask("b", false, Ongoing))
	c := insert(s.InsertTask("c", false, Ongoing))

	ok(t, s.RepositionTask(c.ID, Ongoing, a.ID))
	if got := titles(t, s, Ongoing); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRepositionOntoItselfIsNoop(t *testing.T) {
	s := newTestStore(t)
	insert := must[Task](t)
	insert(s.InsertTask("a", false, Ongoing))
	b := insert(s.InsertTask("b", false, Ongoing))
	insert(s.InsertTask("c", false, Ongoing))

	ok(t, s.RepositionTask(b.ID, Ongoing, b.ID))
	if got := titles(t, s, Ongoing); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRenameKeepsPositionAndColumn(t *testing.T) {
	s := newTestStore(t)
	insert := must[Task](t)
	first := insert(s.InsertTask("first", false, Ongoing))
	insert(s.InsertTask("second", false, Ongoing))

	ok(t, s.UpdateTitle(first.ID, "first, renamed"))
	if got := titles(t, s, Ongoing); !slices.Equal(got, []string{"second", "first, renamed"}) {
		t.Fatalf("got %v", got)
	}
}

func TestTogglePriority(t *testing.T) {
	s := newTestStore(t)
	task := must[Task](t)(s.InsertTask("x", false, Inbox))
	if !must[bool](t)(s.TogglePriority(task.ID)) {
		t.Fatal("first toggle should set priority")
	}
	if must[bool](t)(s.TogglePriority(task.ID)) {
		t.Fatal("second toggle should clear it")
	}
}

func TestUnknownColumnFallsBackToInbox(t *testing.T) {
	if got := parseColumn("someday"); got != Inbox {
		t.Fatalf("got %v", got)
	}
}

func TestOpenCreatesMissingDatabase(t *testing.T) {
	path := t.TempDir() + "/nested/todo.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertTask("first", false, Inbox); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Opening it again finds the task, not a fresh schema.
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := must[[]Task](t)(s.ActiveTasks(Inbox)); len(got) != 1 {
		t.Fatalf("got %d tasks", len(got))
	}
}
