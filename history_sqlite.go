package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// historySchema is the shape of history.db. A file from a newer version is
// refused rather than half-read; the app then runs without history.
const historySchema = 1

// sqliteHistory is the version list: one row per saved library, the whole
// library in each, so a restore needs no replay and no other table. It is
// the only database in the app, it is local, and it is never the library.
type sqliteHistory struct {
	db *sqlite.DB
}

// openSQLiteHistory opens the history database, creating the schema when the
// file is new. The native module may be missing (a build for a system it
// does not cover, or a development download that failed): the caller keeps
// history off and says so.
func openSQLiteHistory(ctx context.Context, path string) (*sqliteHistory, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	h := &sqliteHistory{db: db}
	if err := h.ready(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return h, nil
}

// ready checks the schema version and creates the table when the file is
// new.
func (h *sqliteHistory) ready(ctx context.Context) error {
	rows, err := h.db.Query(ctx, "PRAGMA user_version")
	if err != nil {
		return err
	}
	version := 0
	if len(rows.Values) == 1 && len(rows.Values[0]) == 1 {
		if n, ok := rows.Values[0][0].(int64); ok {
			version = int(n)
		}
	}
	switch {
	case version > historySchema:
		return fmt.Errorf("history: version %d, want %d or older", version, historySchema)
	case version == historySchema:
		return nil
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at INTEGER NOT NULL,
			kind TEXT NOT NULL,
			note TEXT NOT NULL,
			songs INTEGER NOT NULL,
			hash TEXT NOT NULL,
			library TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS revisions_at ON revisions(at DESC)`,
		`PRAGMA user_version = 1`,
	} {
		if _, err := h.db.Execute(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (h *sqliteHistory) Add(ctx context.Context, r revision, library []byte) error {
	_, err := h.db.Execute(ctx,
		`INSERT INTO revisions(at, kind, note, songs, hash, library) VALUES (?, ?, ?, ?, ?, ?)`,
		r.At.UnixMilli(), r.Kind, r.Note, int64(r.Songs), r.Hash, string(library))
	return err
}

func (h *sqliteHistory) Recent(ctx context.Context, limit int) ([]revision, error) {
	rows, err := h.db.Query(ctx,
		`SELECT id, at, kind, note, songs, hash FROM revisions ORDER BY id DESC LIMIT ?`, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]revision, 0, len(rows.Values))
	for _, v := range rows.Values {
		if len(v) != 6 {
			return nil, fmt.Errorf("history: row with %d columns", len(v))
		}
		id, _ := v[0].(int64)
		at, _ := v[1].(int64)
		kind, _ := v[2].(string)
		note, _ := v[3].(string)
		songs, _ := v[4].(int64)
		hash, _ := v[5].(string)
		out = append(out, revision{
			ID:    id,
			At:    time.UnixMilli(at),
			Kind:  kind,
			Note:  note,
			Songs: int(songs),
			Hash:  hash,
		})
	}
	return out, nil
}

func (h *sqliteHistory) Library(ctx context.Context, id int64) ([]byte, error) {
	rows, err := h.db.Query(ctx, `SELECT library FROM revisions WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(rows.Values) == 0 || len(rows.Values[0]) != 1 {
		return nil, fmt.Errorf("history: no version %d", id)
	}
	text, ok := rows.Values[0][0].(string)
	if !ok {
		return nil, errors.New("history: the version is not text")
	}
	return []byte(text), nil
}

func (h *sqliteHistory) Prune(ctx context.Context, keep int) error {
	_, err := h.db.Execute(ctx,
		`DELETE FROM revisions WHERE id NOT IN (SELECT id FROM revisions ORDER BY id DESC LIMIT ?)`,
		int64(keep))
	return err
}

func (h *sqliteHistory) Close() error {
	return h.db.Close()
}
