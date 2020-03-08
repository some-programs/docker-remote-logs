package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/go-pa/fenv"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thomasf/docker-remote-logs/docker"
)

type handler struct {
	client *docker.Client
}

func main() {
	var (
		addr = ""
	)
	flag.StringVar(&addr, "addr", ":8080", "http service address")
	fenv.Prefix("DRLOG_")
	fenv.MustParse()
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()
	dockerClient := docker.NewClient()
	{
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_, err := dockerClient.ListContainers(ctx)
		if err != nil {
			log.Fatalf("Could not connect to Docker Engine: %v", err)
		}
	}

	http.Handle("/metrics", promhttp.Handler())
	h := &handler{dockerClient}
	http.HandleFunc("/api/containers", h.listContainers)
	http.HandleFunc("/api/logs/stream", h.streamLogs)
	http.HandleFunc("/api/logs/download", h.downloadLogs)
	http.HandleFunc("/api/logs/zip", h.downloadZip)
	// http.HandleFunc("/api/events/stream", h.streamEvents)
	http.HandleFunc("/containers", h.container)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", h.index)
	srv := &http.Server{Addr: addr, Handler: http.DefaultServeMux}
	go func() {
		log.Printf("Accepting connections on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("server closed")
				return
			}
			log.Fatal(err)
		}
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)
	<-c
	log.Printf("Shutting down...")
	srv.Close()
}

func (h *handler) listContainers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	containers, err := h.client.ListContainers(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(containers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var (
	streamLogsUpgrader = websocket.Upgrader{
		ReadBufferSize:  10 * 1024,
		WriteBufferSize: 10 * 1024,
	}
)

func (h *handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ws, err := streamLogsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	opts := getLogsOptions(r)
	messages, errCh := h.client.ContainerLogs(ctx, id, opts)
	log.Printf("Starting to stream logs for %s", id)
Loop:
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				break Loop
			}
			e := ws.WriteMessage(websocket.TextMessage, []byte(message))
			if e != nil {
				log.Printf("Error while writing to log stream: %v", e)
				break Loop
			}
		case e := <-errCh:
			log.Printf("Error while reading from log stream: %v", e)
			break Loop
		case <-ctx.Done():
			log.Println("client closed connection")
			return
		}
	}
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

func (h *handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	messages, err := h.client.Events(ctx)

Loop:
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				break Loop
			}
			switch message.Action {
			case "connect", "disconnect", "create", "destroy", "start", "stop":
				log.Printf("Triggering docker event: %v", message.Action)
				_, err := fmt.Fprintf(w, "event: containers-changed\ndata: %s\n\n", message.Action)

				if err != nil {
					log.Printf("Error while writing to event stream: %v", err)
					break
				}
				f.Flush()
			default:
				log.Printf("Ignoring docker event: %v", message.Action)
			}
		case <-ctx.Done():
			break Loop
		case <-err:
			break Loop
		}
	}
}

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

func mustReadFile(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

var (
	indexTemplate = mustReadFile("templates/index.html")
)

func (h *handler) container(w http.ResponseWriter, r *http.Request) {
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

var (
	containerTemplate = mustReadFile("templates/container.html")
)

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
