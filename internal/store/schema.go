package store

const schema = `
CREATE TABLE tasks (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	column_type TEXT NOT NULL,
	is_priority INTEGER NOT NULL DEFAULT 0,
	is_completed INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	completed_at INTEGER,
	sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

// OpenInMemory is for tests: an empty database with the schema.
func OpenInMemory() (*Store, error) {
	s, err := open("file::memory:")
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(schema); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
