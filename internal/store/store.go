// Package store reads and writes the todo database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Column string

const (
	Inbox   Column = "inbox"
	Ongoing Column = "ongoing"
)

// An unrecognised column falls back to Inbox rather than dropping the row, so
// a hand-edited database still opens.
func parseColumn(s string) Column {
	if s == string(Ongoing) {
		return Ongoing
	}
	return Inbox
}

type Task struct {
	ID          string
	Title       string
	Column      Column
	IsPriority  bool
	IsCompleted bool
	CreatedAt   int64
	CompletedAt *int64
}

type Store struct {
	db *sql.DB
}

// Open creates the database, with its schema, when the file does not exist yet.
func Open(path string) (*Store, error) {
	_, err := os.Stat(path)
	fresh := errors.Is(err, os.ErrNotExist)
	if err != nil && !fresh {
		return nil, err
	}
	if fresh {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	dsn := fmt.Sprintf("file:%s?mode=rwc&_pragma=busy_timeout(3000)&_pragma=synchronous(NORMAL)", path)
	s, err := open(dsn)
	if err != nil || !fresh {
		return s, err
	}
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL;" + schema); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection keeps every statement on the same session, which is what
	// an in-memory test database needs and costs nothing for a single window.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) InsertTask(title string, isPriority bool, column Column) (Task, error) {
	t := Task{
		ID:         uuid.NewString(),
		Title:      title,
		Column:     column,
		IsPriority: isPriority,
		CreatedAt:  time.Now().UnixMilli(),
	}
	_, err := s.db.Exec(`
		INSERT INTO tasks (id, title, column_type, is_priority, is_completed, created_at, sort_order)
		VALUES (?, ?, ?, ?, 0, ?, (
			SELECT COALESCE(MIN(sort_order), 0) - 1 FROM tasks
			WHERE column_type = ? AND is_completed = 0
		))`,
		t.ID, t.Title, string(column), isPriority, t.CreatedAt, string(column))
	return t, err
}

// RepositionTask moves id into column, directly above before (or to the end
// when before is empty), renumbering that column.
//
// The target is an anchor id rather than a position, so a row dragged downward
// within its own column cannot land one slot off once it is lifted out.
func (s *Store) RepositionTask(id string, column Column, before string) error {
	// Without this the row is lifted out before the anchor is looked up, the
	// anchor is gone, and the fallback appends it to the end.
	if before == id {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT id FROM tasks
		WHERE column_type = ? AND is_completed = 0
		ORDER BY sort_order ASC`, string(column))
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var existing string
		if err := rows.Scan(&existing); err != nil {
			rows.Close()
			return err
		}
		if existing != id {
			ids = append(ids, existing)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	index := len(ids)
	for i, existing := range ids {
		if existing == before {
			index = i
			break
		}
	}
	ids = append(ids[:index], append([]string{id}, ids[index:]...)...)

	if _, err := tx.Exec(`UPDATE tasks SET column_type = ? WHERE id = ?`, string(column), id); err != nil {
		return err
	}
	for position, taskID := range ids {
		if _, err := tx.Exec(`UPDATE tasks SET sort_order = ? WHERE id = ?`, position, taskID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) MoveTaskColumn(id string, column Column) error {
	return s.RepositionTask(id, column, "")
}

func (s *Store) TogglePriority(id string) (bool, error) {
	var now bool
	err := s.db.QueryRow(`
		UPDATE tasks SET is_priority = 1 - is_priority WHERE id = ?
		RETURNING is_priority`, id).Scan(&now)
	return now, err
}

// UpdateTitle never clears a title: the caller rejects an empty edit and the
// old one stands, because deleting has its own path.
func (s *Store) UpdateTitle(id, title string) error {
	_, err := s.db.Exec(`UPDATE tasks SET title = ? WHERE id = ?`, title, id)
	return err
}

func (s *Store) CompleteTask(id string) error {
	_, err := s.db.Exec(`UPDATE tasks SET is_completed = 1, completed_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), id)
	return err
}

func (s *Store) RestoreTask(id string) error {
	_, err := s.db.Exec(`UPDATE tasks SET is_completed = 0, completed_at = NULL WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (s *Store) ClearCompleted() error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE is_completed = 1`)
	return err
}

const taskColumns = `id, title, column_type, is_priority, is_completed, created_at, completed_at`

func (s *Store) ActiveTasks(column Column) ([]Task, error) {
	return s.queryTasks(`SELECT `+taskColumns+` FROM tasks
		WHERE column_type = ? AND is_completed = 0
		ORDER BY sort_order ASC`, string(column))
}

func (s *Store) CompletedTasks() ([]Task, error) {
	return s.queryTasks(`SELECT ` + taskColumns + ` FROM tasks
		WHERE is_completed = 1
		ORDER BY completed_at DESC`)
}

func (s *Store) queryTasks(query string, args ...any) ([]Task, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		var column string
		var completedAt sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Title, &column, &t.IsPriority, &t.IsCompleted, &t.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		t.Column = parseColumn(column)
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Int64
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
