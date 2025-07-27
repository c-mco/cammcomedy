package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
)

type Comic struct {
	Name  string `json:"name"`
	Date  string `json:"date"`
	Paid  bool   `json:"paid"`
	Notes string `json:"notes"`
}

var comics []Comic

func loadComics() {
	file, err := os.ReadFile("data/comics.json")
	if err != nil {
		log.Println("No existing data file, starting fresh.")
		comics = []Comic{}
		return
	}
	json.Unmarshal(file, &comics)
}

func saveComics() {
	data, _ := json.MarshalIndent(comics, "", "  ")
	os.WriteFile("data/comics.json", data, 0644)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		comics = append(comics, Comic{
			Name:  r.FormValue("name"),
			Date:  r.FormValue("date"),
			Paid:  r.FormValue("paid") == "on",
			Notes: r.FormValue("notes"),
		})
		saveComics()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, comics)
}

func main() {
	loadComics()
	http.HandleFunc("/", indexHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Println("🚀 cammcomedy running at http://localhost:8101")
	http.ListenAndServe(":8101", nil)
}
