package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

var URL string = ":8080"
var MARKDOWN_PATH string = "markdown"
var TEMPLATE_PATH string = "templates"
var NOT_FOUND string = "404"
var markdown goldmark.Markdown
var templates map[string]*template.Template = map[string]*template.Template{}

type Page struct {
	Title       string
	Description string
	Body        template.HTML
}

type BlogPage struct {
	Title       string
	Description string
	Posts       []Post
}

type Post struct {
	Title string
	File  string
}

func toPath(file string) string {
	return path.Join(MARKDOWN_PATH, file+".md")
}

func handler(w http.ResponseWriter, r *http.Request) {
	file := path.Base(r.URL.Path)
	if file == "/" || file == "" {
		file = "index" // Default to index if no specific file is requested
	}

	serveMarkdown(w, file)
}

func serveMarkdown(w http.ResponseWriter, file string) {
	source, err := ioutil.ReadFile(toPath(file))
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	var buf bytes.Buffer
	context := parser.NewContext()
	if err := markdown.Convert([]byte(source), &buf, parser.WithContext(context)); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	metadata := meta.Get(context)
	title := metadata["title"]
	description := metadata["description"]
	page := &Page{
		Title:       fmt.Sprintf("%v", title),
		Description: fmt.Sprintf("%v", description),
		Body:        template.HTML(buf.String()),
	}

	tpl, ok := templates[fmt.Sprintf("%v", metadata["template"])]
	if ok {
		tpl.Execute(w, page)
	} else {
		http.Error(w, "Template not found", http.StatusInternalServerError)
	}
}

func blogHandler(w http.ResponseWriter, r *http.Request) {
	var posts []Post
	excludedFiles := map[string]bool{
		"about":   true,
		"contact": true,
		"404":     true,
	}

	err := filepath.Walk(MARKDOWN_PATH, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			fileName := strings.TrimSuffix(info.Name(), ".md")
			// Check if the file is in the excluded list
			if _, excluded := excludedFiles[fileName]; excluded {
				return nil
			}

			title, _ := ioutil.ReadFile(fullPath)
			posts = append(posts, Post{
				Title: strings.Split(string(title), "\n")[1], // Assuming title is the first line
				File:  fileName,
			})
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Unable to read blog posts", http.StatusInternalServerError)
		return
	}

	page := BlogPage{
		Title:       "Blog",
		Description: "List of blog posts",
		Posts:       posts,
	}

	tpl, ok := templates["blog-list"]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	tpl.Execute(w, page)
}

func main() {
	err := filepath.Walk(
		TEMPLATE_PATH,
		func(fullPath string, info os.FileInfo, err error) error {
			file := path.Base(fullPath)
			parts := strings.Split(file, ".")
			if len(parts) == 2 {
				templates[parts[0]] = template.Must(template.ParseFiles(fullPath))
			}
			return nil
		})

	if err != nil {
		panic(err)
	}

	markdown = goldmark.New(goldmark.WithExtensions(meta.Meta))
	http.HandleFunc("/", handler)         // Dynamic handler for all Markdown files
	http.HandleFunc("/blog", blogHandler) // Blog list handler

	fmt.Println("server is running on url " + URL)
	http.ListenAndServe(URL, nil)
}
