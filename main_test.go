package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
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

	if _, err := db.Query("SELECT fee, paid FROM lineup LIMIT 1"); err != nil {
		t.Errorf("expected columns fee and paid in lineup: %v", err)
	}
}

func TestFetchComics(t *testing.T) {
	setupTestDB(t)

	// empty initially
	comics, err := fetchComics()
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

	comics, err = fetchComics()
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

func addTestEvent(t *testing.T) (eventID int64) {
	res, err := db.Exec("INSERT INTO gigs(name) VALUES('Gig')")
	if err != nil {
		t.Fatalf("insert gig: %v", err)
	}
	gid, _ := res.LastInsertId()
	res, err = db.Exec("INSERT INTO events(gig_id, date, time) VALUES(?,?,?)", gid, "2024-01-01", "20:00")
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	eventID, _ = res.LastInsertId()
	return
}

func addComic(t *testing.T, name string) int64 {
	res, err := db.Exec("INSERT INTO comics(name) VALUES(?)", name)
	if err != nil {
		t.Fatalf("insert comic: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestEventRoleDropdown(t *testing.T) {
	setupTestDB(t)
	eid := addTestEvent(t)

	req := httptest.NewRequest(http.MethodGet, "/event?id="+strconv.FormatInt(eid, 10), nil)
	w := httptest.NewRecorder()
	eventHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<select name=\"role\"") || !strings.Contains(body, "MC") {
		t.Fatalf("role dropdown missing")
	}
}

func TestDuplicateMCNotAllowed(t *testing.T) {
	setupTestDB(t)
	eid := addTestEvent(t)
	c1 := addComic(t, "One")
	c2 := addComic(t, "Two")

	// first MC
	form := url.Values{"comic_id": {strconv.FormatInt(c1, 10)}, "role": {"MC"}}
	req := httptest.NewRequest(http.MethodPost, "/event?id="+strconv.FormatInt(eid, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	eventHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", w.Code)
	}

	// second MC should fail
	form.Set("comic_id", strconv.FormatInt(c2, 10))
	req = httptest.NewRequest(http.MethodPost, "/event?id="+strconv.FormatInt(eid, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	eventHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDuplicateHeadlinerNotAllowed(t *testing.T) {
	setupTestDB(t)
	eid := addTestEvent(t)
	c1 := addComic(t, "H1")
	c2 := addComic(t, "H2")

	form := url.Values{"comic_id": {strconv.FormatInt(c1, 10)}, "role": {"HEADLINER"}}
	req := httptest.NewRequest(http.MethodPost, "/event?id="+strconv.FormatInt(eid, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	eventHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", w.Code)
	}

	form.Set("comic_id", strconv.FormatInt(c2, 10))
	req = httptest.NewRequest(http.MethodPost, "/event?id="+strconv.FormatInt(eid, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	eventHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
