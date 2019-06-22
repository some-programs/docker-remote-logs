package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

// NewClient creates a new instance of Client
func NewClient() *Client {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{cli}
}

func (d *Client) ListContainers() ([]Container, error) {
	list, err := d.cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	var containers []Container
	for _, c := range list {

		container := Container{
			ID:      c.ID[:12],
			Names:   c.Names,
			Name:    strings.TrimPrefix(c.Names[0], "/"),
			Image:   c.Image,
			ImageID: c.ImageID,
			Command: c.Command,
			Created: c.Created,
			State:   c.State,
			Status:  c.Status,
		}
		containers = append(containers, container)
	}

	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Name < containers[j].Name
	})

	if containers == nil {
		containers = []Container{}
	}

	return containers, nil
}

func (d *Client) WriteContainerLog(ctx context.Context, w io.Writer, id string, options types.ContainerLogsOptions) error {
	options.Follow = false
	// options := types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: false, Timestamps: false}
	reader, err := d.cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return err
	}
	defer reader.Close()
	hdr := make([]byte, 8)
	for {
		_, err := reader.Read(hdr)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		count := binary.BigEndian.Uint32(hdr[4:])
		_, err = io.CopyN(w, reader, int64(count))
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	return nil
}
func (d *Client) ContainerLogs(ctx context.Context, id string, options types.ContainerLogsOptions) (<-chan string, <-chan error) {
	reader, err := d.cli.ContainerLogs(ctx, id, options)
	errChannel := make(chan error, 1)

	if err != nil {
		errChannel <- err
		close(errChannel)
		return nil, errChannel
	}

	messages := make(chan string)
	go func() {
		<-ctx.Done()
		reader.Close()
	}()

	go func() {
		defer close(messages)
		defer close(errChannel)
		defer reader.Close()

		hdr := make([]byte, 8)
		var buffer bytes.Buffer
		for {
			_, err := reader.Read(hdr)
			if err != nil {
				errChannel <- err
				break
			}
			count := binary.BigEndian.Uint32(hdr[4:])
			_, err = io.CopyN(&buffer, reader, int64(count))
			if err != nil {
				errChannel <- err
				break
			}
			select {
			case messages <- buffer.String():
			case <-ctx.Done():
			}
			buffer.Reset()
		}
	}()

	return messages, errChannel
}

func (d *Client) Events(ctx context.Context) (<-chan events.Message, <-chan error) {
	return d.cli.Events(ctx, types.EventsOptions{})
}
