package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

func TestSQLiteHistoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	h, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Skipf("SQLite history is unavailable here: %v", err)
	}
	at := time.Date(2026, 11, 24, 9, 12, 0, 0, time.UTC)
	for i, lib := range []string{`{"one":1}`, `{"two":2}`, `{"three":3}`} {
		r := revision{At: at.Add(time.Duration(i) * time.Minute), Kind: "auto", Note: "Saved", Songs: i + 1, Hash: hashBytes([]byte(lib))}
		if err := h.Add(ctx, r, []byte(lib)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := h.Recent(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("recent = %d rows, want 2", len(rows))
	}
	if rows[0].Songs != 3 || rows[1].Songs != 2 {
		t.Errorf("order = %d, %d; want newest first", rows[0].Songs, rows[1].Songs)
	}
	if !rows[0].At.Equal(at.Add(2 * time.Minute)) {
		t.Errorf("time = %v, want %v", rows[0].At, at.Add(2*time.Minute))
	}
	lib, err := h.Library(ctx, rows[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(lib) != `{"two":2}` {
		t.Errorf("library = %q", lib)
	}
	if err := h.Prune(ctx, 1); err != nil {
		t.Fatal(err)
	}
	rows, err = h.Recent(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Songs != 3 {
		t.Fatalf("after prune = %+v, want only the newest", rows)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	rows, err = again.Recent(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("reopened history = %d rows, want 1", len(rows))
	}
}

func TestSQLiteHistoryRefusesANewerSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	h, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Skipf("SQLite history is unavailable here: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Execute(ctx, "PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openSQLiteHistory(ctx, path); err == nil {
		t.Fatal("a newer history schema was accepted")
	}
}
