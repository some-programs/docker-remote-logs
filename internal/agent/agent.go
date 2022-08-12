package agent

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-pa/fenv"
	"github.com/google/subcommands"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thomasf/docker-remote-logs/assets"
	"github.com/thomasf/docker-remote-logs/internal/docker"
)

type AgentCmd struct {
	addr string
}

func (*AgentCmd) Name() string     { return "agent" }
func (*AgentCmd) Synopsis() string { return "agent ..." }
func (*AgentCmd) Usage() string {
	return ``
}

func (p *AgentCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.addr, "addr", ":8080", "http service address")

	es := fenv.NewEnvSet(f, fenv.Prefix("DRLOGS_AGENT_"))
	if err := es.Parse(); err != nil {
		log.Println(err)
	}
}

func (p *AgentCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	err := p.run(ctx)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func (p *AgentCmd) run(ctx context.Context) error {

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
	http.HandleFunc("/api/events/stream", h.streamEvents)

	http.HandleFunc("/logs", h.logs)
	http.HandleFunc("/events", h.event)

	http.Handle("/static/", http.FileServer(http.FS(assets.Static)))
	http.HandleFunc("/", h.index)
	srv := &http.Server{Addr: p.addr, Handler: http.DefaultServeMux}
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

	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	<-ctx.Done()
	log.Printf("Shutting down...")
	srv.Close()
	return nil
}

type handler struct {
	client *docker.Client
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  10 * 1024,
		WriteBufferSize: 10 * 1024,
	}
)

const (
	pongWait   = time.Minute
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
)

func mustReadFile(path string) string {
	data, err := fs.ReadFile(assets.Templates, path)
	if err != nil {
		panic(err)
	}
	return string(data)
}
