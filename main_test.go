package main

import (
	"database/sql"
	"testing"
)

func setupTestDB(t *testing.T) {
	var err error
	// Use shared in-memory database so all connections see the same data
	db, err = sql.Open("sqlite3", "file:test.db?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := initDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
}

func TestInitDBCreatesTables(t *testing.T) {
	setupTestDB(t)

	// Verify we can query expected tables
	tables := []string{"gigs", "events", "comics", "lineup"}
	for _, tbl := range tables {
		if _, err := db.Query("SELECT * FROM " + tbl + " LIMIT 1"); err != nil {
			t.Errorf("expected table %s to exist: %v", tbl, err)
		}
	}
}

func TestFetchComics(t *testing.T) {
	setupTestDB(t)

	// empty initially
	comics, err := fetchComics("")
	if err != nil {
		t.Fatalf("fetchComics failed: %v", err)
	}
	if len(comics) != 0 {
		t.Fatalf("expected 0 comics, got %d", len(comics))
	}

	// insert unsorted names
	if _, err := db.Exec("INSERT INTO comics(name) VALUES('Zed'),('Anna')"); err != nil {
		t.Fatalf("insert comics: %v", err)
	}

	comics, err = fetchComics("")
	if err != nil {
		t.Fatalf("fetchComics failed: %v", err)
	}
	if len(comics) != 2 {
		t.Fatalf("expected 2 comics, got %d", len(comics))
	}
	if comics[0].Name != "Anna" || comics[1].Name != "Zed" {
		t.Fatalf("comics not sorted by name: %#v", comics)
	}
}
