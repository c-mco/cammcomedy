package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))
var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite3", filepath.Join("data", "app.db"))
	if err != nil {
		log.Fatal(err)
	}
	if err := initDB(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", gigsHandler)
	http.HandleFunc("/gig", gigHandler)
	http.HandleFunc("/event", eventHandler)
	http.HandleFunc("/comics", comicsHandler)
	http.HandleFunc("/comic", comicHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("🚀 cammcomedy running at http://localhost:8101")
	log.Fatal(http.ListenAndServe(":8101", nil))
}

func initDB() error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS gigs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            recurrence TEXT
        );
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            gig_id INTEGER NOT NULL,
            date TEXT NOT NULL,
            time TEXT NOT NULL,
            timeline TEXT,
            FOREIGN KEY(gig_id) REFERENCES gigs(id)
        );
        CREATE TABLE IF NOT EXISTS comics (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            bio TEXT,
            notes TEXT,
            contact TEXT,
            default_fee TEXT
        );
        CREATE TABLE IF NOT EXISTS lineup (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            event_id INTEGER NOT NULL,
            comic_id INTEGER NOT NULL,
            role TEXT NOT NULL,
            position INTEGER,
            fee TEXT,
            paid INTEGER DEFAULT 0,
            FOREIGN KEY(event_id) REFERENCES events(id),
            FOREIGN KEY(comic_id) REFERENCES comics(id)
        );`)
	if err != nil {
		return err
	}

	// add new columns for existing installations
	alters := []string{
		"ALTER TABLE comics ADD COLUMN bio TEXT",
		"ALTER TABLE comics ADD COLUMN notes TEXT",
		"ALTER TABLE comics ADD COLUMN contact TEXT",
		"ALTER TABLE comics ADD COLUMN default_fee TEXT",
		"ALTER TABLE lineup ADD COLUMN fee TEXT",
		"ALTER TABLE lineup ADD COLUMN paid INTEGER DEFAULT 0",
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}

type Gig struct {
	ID         int
	Name       string
	Recurrence string
}

type Event struct {
	ID    int
	GigID int
	Date  string
	Time  string
	Notes string
}

func (e Event) Name() string {
	t, err := time.Parse("2006-01-02 15:04", e.Date+" "+e.Time)
	if err != nil {
		return e.Date + " " + e.Time
	}
	return t.Format("Jan 2, 2006 3:04 PM")
}

type Comic struct {
	ID         int
	Name       string
	Bio        string
	Notes      string
	Contact    string
	DefaultFee string
}

type EventDisplay struct {
	Event
	MC        string
	Headliner string
	Comics    [6]string
}

func gigsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		recurrence := r.FormValue("recurrence")
		if name != "" {
			_, err := db.Exec("INSERT INTO gigs(name, recurrence) VALUES(?, ?)", name, recurrence)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	rows, err := db.Query("SELECT id, name, recurrence FROM gigs ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var gigs []Gig
	for rows.Next() {
		var g Gig
		rows.Scan(&g.ID, &g.Name, &g.Recurrence)
		gigs = append(gigs, g)
	}
	templates.ExecuteTemplate(w, "index.html", gigs)
}

func gigHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodPost {
		date := r.FormValue("date")
		time := r.FormValue("time")
		if date != "" && time != "" {
			_, err := db.Exec("INSERT INTO events(gig_id, date, time) VALUES(?,?,?)", id, date, time)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}

	var gig Gig
	err := db.QueryRow("SELECT id, name, recurrence FROM gigs WHERE id=?", id).Scan(&gig.ID, &gig.Name, &gig.Recurrence)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// fetch events
	rows, err := db.Query("SELECT id, date, time, timeline FROM events WHERE gig_id=? ORDER BY date", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var events []EventDisplay
	for rows.Next() {
		var e EventDisplay
		var notes sql.NullString
		rows.Scan(&e.ID, &e.Date, &e.Time, &notes)
		e.Notes = notes.String
		e.GigID = gig.ID
		// fetch lineup
		lrows, _ := db.Query("SELECT comics.name, role, position FROM lineup JOIN comics ON lineup.comic_id = comics.id WHERE event_id=? ORDER BY role, position", e.ID)
		var comics [6]string
		for lrows.Next() {
			var name, role string
			var pos sql.NullInt64
			lrows.Scan(&name, &role, &pos)
			switch role {
			case "MC":
				e.MC = name
			case "HEADLINER":
				e.Headliner = name
			default:
				idx := 0
				if pos.Valid {
					idx = int(pos.Int64) - 1
				} else {
					for i, c := range comics {
						if c == "" {
							idx = i
							break
						}
					}
				}
				if idx >= 0 && idx < len(comics) {
					comics[idx] = name
				}
			}
		}
		lrows.Close()
		e.Comics = comics
		events = append(events, e)
	}

	templates.ExecuteTemplate(w, "gig.html", struct {
		Gig    Gig
		Events []EventDisplay
	}{gig, events})
}

func eventHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if r.FormValue("update_notes") != "" {
			notes := r.FormValue("notes")
			_, err := db.Exec("UPDATE events SET timeline=? WHERE id=?", notes, id)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}

		if lid := r.FormValue("lineup_id"); lid != "" {
			fee := r.FormValue("fee")
			paid := 0
			if r.FormValue("paid") != "" {
				paid = 1
			}
			_, err := db.Exec("UPDATE lineup SET fee=?, paid=? WHERE id=?", fee, paid, lid)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}
		comicID := r.FormValue("comic_id")
		role := r.FormValue("role")
		fee := r.FormValue("fee")
		if comicID != "" && role != "" {
			// disallow multiple MCs or Headliners
			if role == "MC" || role == "HEADLINER" {
				var cnt int
				db.QueryRow("SELECT COUNT(*) FROM lineup WHERE event_id=? AND role=?", id, role).Scan(&cnt)
				if cnt > 0 {
					http.Error(w, role+" already assigned", http.StatusBadRequest)
					return
				}
			}

			// find max position for comics
			var pos sql.NullInt64
			if role == "COMIC" {
				db.QueryRow("SELECT COALESCE(MAX(position),0)+1 FROM lineup WHERE event_id=? AND role='COMIC'", id).Scan(&pos)
			}
			_, err := db.Exec("INSERT INTO lineup(event_id, comic_id, role, position, fee) VALUES(?,?,?,?,?)", id, comicID, role, pos.Int64, fee)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}

	var event Event
	var notes sql.NullString
	err := db.QueryRow("SELECT id, gig_id, date, time, timeline FROM events WHERE id=?", id).Scan(&event.ID, &event.GigID, &event.Date, &event.Time, &notes)
	event.Notes = notes.String
	if err != nil {
		log.Println("query error", err)
		http.NotFound(w, r)
		return
	}

	comics, _ := fetchComics()

	// fetch lineup
	lrows, _ := db.Query("SELECT lineup.id, comics.name, role, fee, paid, position FROM lineup JOIN comics ON lineup.comic_id = comics.id WHERE event_id=? ORDER BY role, position", id)
	type LineupItem struct {
		ID   int
		Name string
		Role string
		Fee  string
		Paid bool
	}
	var lineup []LineupItem
	for lrows.Next() {
		var li LineupItem
		var pos sql.NullInt64
		var paid int
		lrows.Scan(&li.ID, &li.Name, &li.Role, &li.Fee, &paid, &pos)
		li.Paid = paid == 1
		lineup = append(lineup, li)
	}
	lrows.Close()

	templates.ExecuteTemplate(w, "event.html", struct {
		Event  Event
		Comics []Comic
		Lineup []LineupItem
	}{event, comics, lineup})
}

func fetchComics() ([]Comic, error) {
	rows, err := db.Query("SELECT id, name, bio, notes, contact, default_fee FROM comics ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comics []Comic
	for rows.Next() {
		var c Comic
		rows.Scan(&c.ID, &c.Name, &c.Bio, &c.Notes, &c.Contact, &c.DefaultFee)
		comics = append(comics, c)
	}
	return comics, nil
}

func fetchComic(id string) (Comic, error) {
	var c Comic
	err := db.QueryRow("SELECT id, name, bio, notes, contact, default_fee FROM comics WHERE id=?", id).Scan(
		&c.ID, &c.Name, &c.Bio, &c.Notes, &c.Contact, &c.DefaultFee)
	return c, err
}

func comicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		bio := r.FormValue("bio")
		notes := r.FormValue("notes")
		contact := r.FormValue("contact")
		fee := r.FormValue("fee")
		if name != "" {
			_, err := db.Exec("INSERT INTO comics(name, bio, notes, contact, default_fee) VALUES(?,?,?,?,?)", name, bio, notes, contact, fee)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, "/comics", http.StatusSeeOther)
		return
	}
	comics, _ := fetchComics()
	templates.ExecuteTemplate(w, "comics.html", comics)
}

func comicHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if r.FormValue("delete") != "" {
			if _, err := db.Exec("DELETE FROM comics WHERE id=?", id); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			http.Redirect(w, r, "/comics", http.StatusSeeOther)
			return
		}
		name := r.FormValue("name")
		bio := r.FormValue("bio")
		notes := r.FormValue("notes")
		contact := r.FormValue("contact")
		fee := r.FormValue("fee")
		_, err := db.Exec("UPDATE comics SET name=?, bio=?, notes=?, contact=?, default_fee=? WHERE id=?", name, bio, notes, contact, fee, id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, "/comics", http.StatusSeeOther)
		return
	}
	comic, err := fetchComic(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	templates.ExecuteTemplate(w, "comic.html", comic)
}
