package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// Comic represents a comedian that can be booked for events.
type Comic struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Contact    string `json:"contact"`
	DefaultFee int    `json:"default_fee"`
	Notes      string `json:"notes"`
}

// EventComic stores event specific info about a comic.
type EventComic struct {
	ComicID int    `json:"comic_id"`
	Fee     int    `json:"fee"`
	Paid    bool   `json:"paid"`
	Notes   string `json:"notes"`
}

// Event represents a single comedy night.
type Event struct {
	ID       int          `json:"id"`
	GigID    int          `json:"gig_id"`
	Date     string       `json:"date"`
	Time     string       `json:"time"`
	Timeline string       `json:"timeline"`
	Comics   []EventComic `json:"comics"`
}

// Gig is a recurring show at a venue.
type Gig struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Venue       string `json:"venue"`
	Address     string `json:"address"`
	Description string `json:"description"`
	Instagram   string `json:"instagram"`
	Contact     string `json:"contact"`
}

// Data is the on-disk representation of the application database.
type Data struct {
	Comics []Comic `json:"comics"`
	Events []Event `json:"events"`
	Gigs   []Gig   `json:"gigs"`
}

var db Data

func dataFile() string {
	return filepath.Join("data", "app.json")
}

func loadData() {
	file, err := os.ReadFile(dataFile())
	if err != nil {
		log.Println("No existing data file, starting fresh.")
		// seed with default gig
		db = Data{
			Gigs: []Gig{{
				ID:          1,
				Name:        "Comedy at CoConspirators",
				Venue:       "CoConspirators Brewing Co.",
				Address:     "Brunswick",
				Description: "Monthly comedy night at CoConspirators",
			}},
		}
		return
	}
	json.Unmarshal(file, &db)
}

func saveData() {
	os.MkdirAll("data", 0755)
	data, _ := json.MarshalIndent(db, "", "  ")
	os.WriteFile(dataFile(), data, 0644)
}

// indexHandler lists events for the first gig and allows adding new ones.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		id := len(db.Events) + 1
		db.Events = append(db.Events, Event{
			ID:       id,
			GigID:    1,
			Date:     r.FormValue("date"),
			Time:     r.FormValue("time"),
			Timeline: r.FormValue("timeline"),
		})
		saveData()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	var events []Event
	maxComics := 0
	for _, e := range db.Events {
		if e.GigID == 1 {
			events = append(events, e)
			if len(e.Comics) > maxComics {
				maxComics = len(e.Comics)
			}
		}
	}

	comicMap := make(map[int]Comic)
	for _, c := range db.Comics {
		comicMap[c.ID] = c
	}

	headers := []string{"MC", "Headliner"}
	for i := 2; i < maxComics; i++ {
		headers = append(headers, "Comic #"+strconv.Itoa(i-1))
	}

	tmpl.Execute(w, struct {
		Gig      Gig
		Events   []Event
		ComicMap map[int]Comic
		Headers  []string
		Indices  []int
	}{db.Gigs[0], events, comicMap, headers, makeRange(maxComics)})
}

// comicsHandler lists comics and allows adding new ones.
func comicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		id := len(db.Comics) + 1
		fee, _ := strconv.Atoi(r.FormValue("fee"))
		db.Comics = append(db.Comics, Comic{
			ID:         id,
			Name:       r.FormValue("name"),
			Contact:    r.FormValue("contact"),
			DefaultFee: fee,
			Notes:      r.FormValue("notes"),
		})
		saveData()
		http.Redirect(w, r, "/comics", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/comics.html"))
	tmpl.Execute(w, db.Comics)
}

// eventHandler manages a single event and its lineup.
func eventHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	var event *Event
	for i := range db.Events {
		if db.Events[i].ID == id {
			event = &db.Events[i]
			break
		}
	}
	if event == nil {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		comicID, _ := strconv.Atoi(r.FormValue("comic_id"))
		fee, _ := strconv.Atoi(r.FormValue("fee"))
		event.Comics = append(event.Comics, EventComic{
			ComicID: comicID,
			Fee:     fee,
			Paid:    r.FormValue("paid") == "on",
			Notes:   r.FormValue("notes"),
		})
		saveData()
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}

	comicMap := make(map[int]Comic)
	for _, c := range db.Comics {
		comicMap[c.ID] = c
	}

	tmpl := template.Must(template.ParseFiles("templates/event.html"))
	tmpl.Execute(w, struct {
		Event    *Event
		Comics   []Comic
		ComicMap map[int]Comic
	}{event, db.Comics, comicMap})
}

func makeRange(n int) []int {
	r := make([]int, n)
	for i := 0; i < n; i++ {
		r[i] = i
	}
	return r
}

func main() {
	loadData()
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/comics", comicsHandler)
	http.HandleFunc("/event", eventHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Println("🚀 cammcomedy running at http://localhost:8101")
	http.ListenAndServe(":8101", nil)
}
