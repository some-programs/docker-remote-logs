package main

import (
	"context"
	"log"
	"net/http"
	"text/template"

	"github.com/thomasf/docker-remote-logs/docker"
)

var (
	indexTemplate = mustReadFile("templates/index.html")
)

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if r.URL.Path != "/" {
		http.Error(w, "not found", 404)
		return
	}
	containers, err := h.client.ListContainers(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// IndexData .
	type IndexData struct {
		Containers []docker.Container
	}
	data := IndexData{
		Containers: containers,
	}
	err = t.Execute(w, data)
	if err != nil {
		log.Println(err)
		return
	}

}
