package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/gorilla/websocket"
)

var (
	eventsTemplate = mustReadFile("templates/events.html")
)

// just for experimentation right now
func (h *handler) event(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	_ = ctx
	t, err := template.New("container").Parse(eventsTemplate)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type Data struct {
		StreamURL string
	}
	data := Data{
		StreamURL: fmt.Sprintf("/api/events/stream"),
	}
	err = t.Execute(w, data)
	if err != nil {
		log.Println(err)
		return
	}
}

// ContainerEvent .
type ContainerEvent struct {
	Status   string
	ID       string
	From     string
	TimeNano int64
}

func NewContainerEvent(e events.Message) ContainerEvent {
	return ContainerEvent{
		Status:   e.Status,
		ID:       e.ID,
		From:     e.From,
		TimeNano: e.TimeNano,
	}

}

func (h *handler) streamEvents(w http.ResponseWriter, r *http.Request) {
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

	messages, errors := h.client.Events(ctx)

	var wg sync.WaitGroup
	ws.SetReadDeadline(time.Now().Add(pongWait))

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
	loop:
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
					log.Println("error receiving message")
					return
				}
				if message.Type != "container" {
					// log.Printf("Ignoring docker type: %v", message.Type)
					continue loop
				}
				switch message.Action {
				case "connect", "disconnect", "create", "destroy", "start", "stop":
					ce := NewContainerEvent(message)
					data, err := json.Marshal(&ce)
					if err != nil {
						log.Println(err)
						return
					}
					data = append(data, byte('\n'))
					if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
						log.Printf("Error while writing to event stream: %v", err)
						return
					}

				default:
					log.Printf("Ignoring docker event: %v", message.Action)
				}
			case <-ctx.Done():
				return
			case <-errors:
				return
			}
		}
	}()

	wg.Wait()
}
