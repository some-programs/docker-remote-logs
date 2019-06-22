package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/go-pa/fenv"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/thomasf/docker-remote-logs/docker"
)

type handler struct {
	client *docker.Client
}

func main() {
	var (
		addr  = ""
		level = ""
	)
	flag.StringVar(&addr, "addr", ":8080", "http service address")
	flag.StringVar(&level, "level", "info", "logging level")
	fenv.Prefix("DRLOG_")
	fenv.MustParse()
	flag.Parse()

	l, _ := log.ParseLevel(level)
	log.SetLevel(l)
	dockerClient := docker.NewClient()
	_, err := dockerClient.ListContainers()

	if err != nil {
		log.Fatalf("Could not connect to Docker Engine: %v", err)
	}
	h := &handler{dockerClient}
	http.HandleFunc("/api/containers", h.listContainers)
	http.HandleFunc("/api/logs/stream", h.streamLogs)
	http.HandleFunc("/api/logs/download", h.downloadLogs)
	// http.HandleFunc("/api/events/stream", h.streamEvents)
	http.HandleFunc("/containers", h.container)
	http.HandleFunc("/", h.index)
	srv := &http.Server{Addr: addr, Handler: http.DefaultServeMux}
	go func() {
		log.Infof("Accepting connections on %s", srv.Addr)
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
	log.Infof("Shutting down...")
	srv.Close()
}

func (h *handler) listContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.client.ListContainers()
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
	strreamLogsUpgrader = websocket.Upgrader{
		ReadBufferSize:  10 * 1024,
		WriteBufferSize: 10 * 1024,
	}
)

func (h *handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	ws, err := strreamLogsUpgrader.Upgrade(w, r, nil)
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
	messages, errCh := h.client.ContainerLogs(r.Context(), id, opts)
	log.Debugf("Starting to stream logs for %s", id)
Loop:
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				break Loop
			}
			e := ws.WriteMessage(websocket.TextMessage, []byte(message))
			if e != nil {
				log.Debugf("Error while writing to log stream: %v", e)
				break Loop
			}
		case e := <-errCh:
			log.Debugf("Error while reading from log stream: %v", e)
			break Loop
		}
	}
	log.WithField("NumGoroutine", runtime.NumGoroutine()).Debug("runtime stats")
}

func (h *handler) downloadLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	dockerClient := docker.NewClient()
	containers, err := dockerClient.ListContainers()
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

	if err := h.client.WriteContainerLog(r.Context(), w, id); err != nil {
		log.Printf("error creating download: %v", err)
	}
}

func (h *handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	ctx := r.Context()
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
				log.Debugf("Triggering docker event: %v", message.Action)
				_, err := fmt.Fprintf(w, "event: containers-changed\ndata: %s\n\n", message.Action)

				if err != nil {
					log.Debugf("Error while writing to event stream: %v", err)
					break
				}
				f.Flush()
			default:
				log.Debugf("Ignoring docker event: %v", message.Action)
			}
		case <-ctx.Done():
			break Loop
		case <-err:
			break Loop
		}
	}
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "not found", 404)
		return
	}
	containers, err := h.client.ListContainers()
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

const indexTemplate = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>container</title>
<style>
body {
  font-family: 'Roboto Mono', monospace;
  font-size: 10pt;
}
td.nowrap {
  white-space: nowrap;
}
</style>
</head>
<body>
<div>
<p>
| <input type=checkbox id=stdout checked> stdout
| <input type=checkbox id=stderr checked> stderr
| <input type=checkbox id=timestamps> timestamps (download only)
| <input type=checkbox id=follow checked>follow (view only)
</p>
<p>
| tail:<input type="text" id=tail value="300" maxlength="8" size="6">
| since:<input type="text" id=since >
| until:<input type="text" id=until >
</p>
</div>
<script>

function checkbox(name) {
  if (document.getElementById(name).checked ) {
    return "&" + name + "=true"
  }
  return "&" + name + "=false"
}
function text(name) {
  if (document.getElementById(name).value === "") {
    return ""
  }
  return "&" + name + "=" + document.getElementById(name).value
}
function viewLogs(id) {
  window.location.href = "/containers?id=" + id + checkbox("stdout") + checkbox("stderr") + checkbox("follow") + text("tail") + text("since") + text("until");
}

function downloadLogs(id) {
  window.location.href = "/api/logs/download?id=" + id + checkbox("stdout") + checkbox("stderr") + checkbox("timestamps") + text("tail") + text("since") + text("until");
}


</script>
<table>
<tr>
<th>Logs</th>
<th>Name</th>
<th>Status</th>
<th>Command</th>
</tr>
{{ range .Containers }}
<tr>
<td class="nowrap">
<button type="button" onclick="viewLogs('{{ .ID }}');">view</button>
<button type="button" onclick="downloadLogs('{{ .ID }}');">dl</button>
</td>
<td class="nowrap">{{ .Name }}</td>
<td class="nowrap">{{ .Status }}</td>
<td>{{ .Command }}</td>
</tr>
{{ end }}
</table>
</body>
</html>`

func (h *handler) container(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	opts := getLogsOptions(r)
	t, err := template.New("container").Parse(containerTemplate)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type Data struct {
		StreamURL  string
		Timestamps bool
	}

	timestamps := opts.Timestamps
	opts.Timestamps = true
	data := Data{
		StreamURL:  fmt.Sprintf("/api/logs/stream?id=%v&%s", id, getLogsOptionsQuery(opts)),
		Timestamps: timestamps,
	}
	err = t.Execute(w, data)
	if err != nil {
		log.Println(err)
		return
	}
}

const containerTemplate = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>index</title>
<link href="https://fonts.googleapis.com/css?family=Roboto+Mono&display=swap" rel="stylesheet">
<style>
body {
  font-family: 'Roboto Mono', monospace;
}
pre, pre * {
  white-space: pre;
  font-size: 9pt;
}

.hidets span {
  display: none;
}
span.ts {
  margin-right: 1em;
  color: #444;
}

button.on {
  background-color: green;
  color: white;
}
button.off {
  background-color: darkgray;
  color: #444;
}
pre.wrap, pre.wrap * {
  white-space: pre-wrap;
}
</style>
</head>
<body>

<p><strong id="interval"></strong></p>
<button type="button" onclick="follow();">⇊follow⇊</button>
<button type="button" id="autoscroll" onclick="toggleScroll();">auto scroll</button>
<button type="button" id="wraplines" onclick="toggleWrap();">line wrapping</button>
<button type="button" id="timestamps" onclick="toggleTimestamps();">timestamps</button>
<pre id="log" class="hidets">
</pre>
</body>
<script>

document.state = {
  'autoscroll': document.getElementById("autoscroll").checked,
  'wraplines': document.getElementById("wraplines").checked,
  'timestamps': document.getElementById("timestamps").checked,
  'ts_low': null,
  'ts_high': null,

};

function updateInterval() {
  var el = document.getElementById("interval");
  el.innerText="" + document.state.ts_low + " - " + document.state.ts_high;
}

function buttonState(name) {
  var elm = document.getElementById(name);
  if (document.state[name]) {
    elm.classList.add("on");
    elm.classList.remove("off");
  } else {
    elm.classList.add("off");
    elm.classList.remove("on");
  }
}

buttonState("autoscroll");
buttonState("wraplines");
buttonState("timestamps");

function follow(){
  document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
  document.state.autoscroll = true;
  buttonState("autoscroll")
}

function toggleTimestamps() {
  document.state.timestamps = !document.state.timestamps;
  var elm = document.getElementById("log");
  if (document.state.timestamps) {
    elm.classList.remove("hidets");
  } else {
    elm.classList.add("hidets");
  }
  buttonState("timestamps")
}

function toggleWrap(){
  document.state.wraplines = !document.state.wraplines;
  if (document.state.wraplines) {
    document.getElementById("log").classList.add("wrap")
  } else {
    document.getElementById("log").classList.remove("wrap")
  }
  buttonState("wraplines");
}

function toggleScroll(){
  document.state.autoscroll = !document.state.autoscroll;
  buttonState("autoscroll")
}

var conn_url="";
if (window.location.protocol === "https:") {
  conn_url = "wss:";
} else {
  conn_url = "ws:";
}

conn_url += "//" + window.location.host + "{{ .StreamURL }}";
var log = document.getElementById("log");
var conn = new WebSocket(conn_url);

conn.onopen = function(e) {
  window.setInterval(updateInterval, 450);
}
conn.onerror = function() {
  conn.close();
};
conn.onclose = function() {
  log.appendChild(document.createTextNode("\n\n-- CONNECTION TO LOG STREAM CLOSED --\n\n"));
};
conn.onmessage = function(e) {
  var ts = e.data.slice(0,30);
  if (document.state.ts_low == null || ts < document.state.ts_low) {
    document.state.ts_low = ts;
  };
  if (document.state.ts_high == null || ts > document.state.ts_high) {
    document.state.ts_high = ts;
  };
  var tse = document.createElement('span');
  tse.textContent = ts;
  tse.setAttribute('class', 'ts')
  log.appendChild(tse)

  log.appendChild(document.createTextNode(e.data.slice(31)))
  if (document.state.autoscroll && (window.innerHeight + window.scrollY) >= document.body.offsetHeight){
    document.scrollingElement.scrollTo(0, document.scrollingElement.scrollHeight);
  };
};
</script>
</html>`

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
