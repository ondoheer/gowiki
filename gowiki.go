// First example

package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/oxtoacart/bpool" // A common use case for this package is to use buffers to execute HTML templates against (via ExecuteTemplate)
	//or encode JSON into (via json.NewEncoder).
	//This allows you to catch any rendering or marshalling errors prior to writing to a http.ResponseWriter,
	//which helps to avoid writing incomplete or malformed data to the response.
)

// var templateBaseDir = "templates"
var dataBaseDir = "data"

var templates map[string]*template.Template
var bufpool *bpool.BufferPool

type Page struct {
	Title string
	Body  []byte
}

type TemplateConfig struct {
	TemplateLayoutPath  string
	TemplateIncludePath string
}

var mainTempl = `{{define "main" }} {{ template "base" . }} {{ end }}`
var templateConfig TemplateConfig

func loadConfiguration() {
	templateConfig.TemplateLayoutPath = "templates/layouts/"
	templateConfig.TemplateIncludePath = "templates/"
}

func loadTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	layoutFiles, err := filepath.Glob(templateConfig.TemplateLayoutPath + "*.html")
	if err != nil {
		log.Fatal(err)
	}

	includeFiles, err := filepath.Glob(templateConfig.TemplateIncludePath + "*.html")
	if err != nil {
		log.Fatal(err)
	}

	mainTemplate := template.New("main")

	mainTemplate, err = mainTemplate.Parse(mainTempl)

	if err != nil {
		log.Fatal(err)
	}

	for _, file := range includeFiles {
		fileName := filepath.Base(file)
		files := append(layoutFiles, file)

		templates[fileName], err = mainTemplate.Clone()

		if err != nil {
			log.Fatal(err)
		}

		templates[fileName] = template.Must(templates[fileName].ParseFiles(files...))
	}
	log.Println("Templates loades successfully")

	bufpool = bpool.NewBufferPool(64)
	log.Println("buffer allocation succesful")

}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := templates[name]

	if !ok {
		http.Error(w, fmt.Sprintf("the template %s does not exist", name),
			http.StatusInternalServerError)
	}

	buf := bufpool.Get()
	defer bufpool.Put(buf)

	err := tmpl.Execute(buf, data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func generateArticlePath(title string) string {
	return filepath.Join(dataBaseDir, title+".txt")
}

// Globals

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func (p *Page) save() error {

	filename := generateArticlePath(p.Title)

	return ioutil.WriteFile(filename, p.Body, 0600)

}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)

	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("Invalid Page Title")
	}

	return m[2], nil // the title is the second subexpression
}

func loadPage(title string) (*Page, error) {

	filename := generateArticlePath(title)

	body, err := ioutil.ReadFile(filename)

	if err != nil {

		return nil, err

	}

	return &Page{Title: title, Body: body}, nil

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.html", nil)
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {

	p, err := loadPage(title)

	// if this page does not exists, go to the editor to create it
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	renderTemplate(w, "view.html", p)

}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {

	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit.html", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {

	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Here we will extract the page title from the Request,
		// and call the provided handler 'fn'
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {

	loadConfiguration()
	loadTemplates()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	http.ListenAndServe(":8080", nil)

}
