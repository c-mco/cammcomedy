package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
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
            name TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS lineup (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            event_id INTEGER NOT NULL,
            comic_id INTEGER NOT NULL,
            role TEXT NOT NULL,
            position INTEGER,
            FOREIGN KEY(event_id) REFERENCES events(id),
            FOREIGN KEY(comic_id) REFERENCES comics(id)
        );`)
	return err
}

type Gig struct {
	ID         int
	Name       string
	Recurrence string
}

type Event struct {
	ID       int
	GigID    int
	Date     string
	Time     string
	Timeline string
}

type Comic struct {
	ID   int
	Name string
}

type EventDisplay struct {
	Event
	MC        string
	Headliner string
	Comics    []string
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
	rows, err := db.Query("SELECT id, date, time FROM events WHERE gig_id=? ORDER BY date", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var events []EventDisplay
	for rows.Next() {
		var e EventDisplay
		rows.Scan(&e.ID, &e.Date, &e.Time)
		e.GigID = gig.ID
		// fetch lineup
		lrows, _ := db.Query("SELECT comics.name, role FROM lineup JOIN comics ON lineup.comic_id = comics.id WHERE event_id=?", e.ID)
		var comics []string
		for lrows.Next() {
			var name, role string
			lrows.Scan(&name, &role)
			switch role {
			case "MC":
				e.MC = name
			case "HEADLINER":
				e.Headliner = name
			default:
				comics = append(comics, name)
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
		comicID := r.FormValue("comic_id")
		role := r.FormValue("role")
		if comicID != "" && role != "" {
			// find max position for comics
			var pos sql.NullInt64
			if role == "COMIC" {
				db.QueryRow("SELECT COALESCE(MAX(position),0)+1 FROM lineup WHERE event_id=? AND role='COMIC'", id).Scan(&pos)
			}
			_, err := db.Exec("INSERT INTO lineup(event_id, comic_id, role, position) VALUES(?,?,?,?)", id, comicID, role, pos.Int64)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}

	var event Event
	err := db.QueryRow("SELECT id, gig_id, date, time FROM events WHERE id=?", id).Scan(&event.ID, &event.GigID, &event.Date, &event.Time)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	comics, _ := fetchComics()

	// fetch lineup
	lrows, _ := db.Query("SELECT lineup.id, comics.name, role, position FROM lineup JOIN comics ON lineup.comic_id = comics.id WHERE event_id=? ORDER BY role, position", id)
	type LineupItem struct {
		ID   int
		Name string
		Role string
	}
	var lineup []LineupItem
	for lrows.Next() {
		var li LineupItem
		var pos sql.NullInt64
		lrows.Scan(&li.ID, &li.Name, &li.Role, &pos)
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
	rows, err := db.Query("SELECT id, name FROM comics ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comics []Comic
	for rows.Next() {
		var c Comic
		rows.Scan(&c.ID, &c.Name)
		comics = append(comics, c)
	}
	return comics, nil
}

func comicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name != "" {
			_, err := db.Exec("INSERT INTO comics(name) VALUES(?)", name)
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
