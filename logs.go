package main

import (
	"archive/zip"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
	"github.com/thomasf/docker-remote-logs/docker"
)

var (
	containerTemplate = mustReadFile("templates/logs.html")
)

func (h *handler) logs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	containers, err := h.client.ListContainers(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	opts := getLogsOptions(r)

	t, err := template.New("container").Parse(containerTemplate)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type Data struct {
		Container  docker.Container
		StreamURL  string
		Timestamps bool
		Containers []docker.Container
	}
	var container docker.Container
	for _, c := range containers {
		if c.ID == id {
			container = c
		}
	}

	timestamps := opts.Timestamps
	opts.Timestamps = true
	data := Data{
		StreamURL:  fmt.Sprintf("/api/logs/stream?id=%v&%s", id, getLogsOptionsQuery(opts)),
		Timestamps: timestamps,
		Containers: containers,
		Container:  container,
	}
	err = t.Execute(w, data)
	if err != nil {
		log.Println(err)
		return
	}
}

func (h *handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	defer ws.Close()
	ws.SetPongHandler(func(string) error {
		// log.Println("pong")
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	opts := getLogsOptions(r)
	messages, errCh := h.client.ContainerLogs(ctx, id, opts)
	log.Printf("Starting to stream logs for %s", id)

	ws.SetReadDeadline(time.Now().Add(pongWait))
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		for {
			// read must be called to get pong messages handeled
			messageType, p, err := ws.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}
			_ = messageType
			_ = p
			// log.Println("read message")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// log.Println("ping")
				err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
				if err != nil {
					log.Println("err", err)
					return
				}
			case message, ok := <-messages:
				if !ok {
					return
				}
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				e := ws.WriteMessage(websocket.TextMessage, []byte(message))
				if e != nil {
					log.Printf("Error while writing to log stream: %v", e)
					return
				}
			case e := <-errCh:
				log.Printf("Error while reading from log stream: %v", e)
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	// log.Println("Stopped streaming log")
}

func (h *handler) downloadLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	dockerClient := docker.NewClient()
	containers, err := dockerClient.ListContainers(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var filename string
	for _, c := range containers {
		if c.ID == id {
			filename = c.Name
		}
	}
	if filename == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.txt", filename))
	opts := getLogsOptions(r)
	if err := h.client.WriteContainerLog(r.Context(), w, id, opts); err != nil {
		log.Printf("error creating download: %v", err)
	}
}

func (h *handler) downloadZip(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	dockerClient := docker.NewClient()
	containers, err := dockerClient.ListContainers(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	opts := getLogsOptions(r)
	filename := fmt.Sprintf("docker-remote-logs.zip")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	z := zip.NewWriter(w)
	defer z.Close()
	for _, c := range containers {
		f, err := z.Create(fmt.Sprintf("%s.txt", c.Name))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := h.client.WriteContainerLog(r.Context(), f, c.ID, opts); err != nil {
			log.Printf("error creating log in archive: %v", err)
			return
		}
	}
}

func getLogsOptionsQuery(opts types.ContainerLogsOptions) string {

	var vs []string
	boolv := map[bool]string{
		true:  "true",
		false: "false",
	}
	vs = append(vs, fmt.Sprintf("stdout=%s", boolv[opts.ShowStdout]))
	vs = append(vs, fmt.Sprintf("stderr=%s", boolv[opts.ShowStderr]))
	vs = append(vs, fmt.Sprintf("timestamps=%s", boolv[opts.Timestamps]))
	vs = append(vs, fmt.Sprintf("follow=%s", boolv[opts.Follow]))
	// vs = append(vs, fmt.Sprintf("details=%s", boolv[opts.Details]))

	if opts.Since != "" {
		vs = append(vs, fmt.Sprintf("since=%s", opts.Since))
	}
	if opts.Until != "" {
		vs = append(vs, fmt.Sprintf("until=%s", opts.Until))
	}

	if opts.Tail != "" {
		vs = append(vs, fmt.Sprintf("tail=%s", opts.Tail))
	}

	return strings.Join(vs, "&")
}

func getLogsOptions(r *http.Request) types.ContainerLogsOptions {
	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "",
		Until:      "",
		Timestamps: false,
		Follow:     false,
		Tail:       "",
		Details:    false,
	}
	for _, p := range []struct {
		n string
		v *bool
	}{
		{"stdout", &opts.ShowStdout},
		{"stderr", &opts.ShowStderr},
		{"timestamps", &opts.Timestamps},
		{"follow", &opts.Follow},
		// {"details", &opts.Details},
	} {
		if v := r.URL.Query().Get(p.n); v != "" {
			r := v == "true" || v == "1"
			*p.v = r
		}
	}
	for _, p := range []struct {
		n string
		v *string
	}{
		{"since", &opts.Since},
		{"until", &opts.Until},
		{"tail", &opts.Tail},
	} {
		if v := r.URL.Query().Get(p.n); v != "" {
			*p.v = v
		}
	}

	return opts
}
