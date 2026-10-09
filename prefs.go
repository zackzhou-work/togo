package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// prefs lives beside the todo database rather than in it. Window geometry is
// kept by mygo's StateKey; this holds only what StateKey does not.
type prefs struct {
	Pinned bool `json:"pinned"`
}

func prefsPath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "prefs.json")
}

// dbPath sits beside prefs.json. TOGO_DB points it elsewhere, for trying a
// build without touching real tasks.
func dbPath() string {
	if path := os.Getenv("TOGO_DB"); path != "" {
		return path
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "todo.db"
	}
	return filepath.Join(dir, "todo.db")
}

func loadPrefs() prefs {
	var p prefs
	if raw, err := os.ReadFile(prefsPath()); err == nil {
		_ = json.Unmarshal(raw, &p)
	}
	return p
}

func savePrefs(p prefs) error {
	path := prefsPath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
