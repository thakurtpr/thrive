//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func dispatch(ctx context.Context, req *Request) *Response {
	switch req.Cmd {
	case "ps":
		return handlePS(ctx, req)
	case "run":
		return handleRun(ctx, req)
	case "logs":
		return handleLogs(ctx, req)
	case "kill":
		return handleKill(ctx, req)
	case "rm":
		return handleRm(ctx, req)
	case "system":
		return handleSystem(ctx, req)
	case "ping":
		return &Response{ID: req.ID, Result: map[string]any{"ok": true}}
	default:
		return &Response{
			ID:    req.ID,
			Error: &ErrorInfo{Code: 1, Message: "unknown command: " + req.Cmd},
		}
	}
}

func handlePS(ctx context.Context, req *Request) *Response {
	containers, err := listContainers()
	if err != nil {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: err.Error()}}
	}
	return &Response{
		ID:     req.ID,
		Result: map[string]any{"containers": containers},
	}
}

func handleRun(ctx context.Context, req *Request) *Response {
	if len(req.Args) < 1 {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: "run requires image argument"}}
	}

	image := req.Args[0]
	cmd := req.Args[1:]

	cfg := runtime.ContainerConfig{
		ID:     generateID(),
		Image:  image,
		Command: cmd,
	}

	if _, err := runtime.Create(ctx, cfg); err != nil {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: err.Error()}}
	}

	detach, _ := req.Opts["detach"].(bool)
	if !detach {
		if err := runtime.Start(ctx, cfg.ID); err != nil {
			return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: err.Error()}}
		}
		log.Printf("thrived: container %s started", cfg.ID)
	} else {
		go func() {
			if err := runtime.Start(ctx, cfg.ID); err != nil {
				log.Printf("thrived: container start error: %v", err)
			} else {
				log.Printf("thrived: container %s started (detached)", cfg.ID)
			}
		}()
	}

	return &Response{
		ID:     req.ID,
		Result: map[string]any{"container_id": cfg.ID},
	}
}

func handleLogs(ctx context.Context, req *Request) *Response {
	if len(req.Args) < 1 {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: "logs requires container ID"}}
	}
	return &Response{
		ID:     req.ID,
		Result: map[string]any{"streaming": true, "container_id": req.Args[0]},
	}
}

func handleKill(ctx context.Context, req *Request) *Response {
	if len(req.Args) < 1 {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: "kill requires container ID"}}
	}
	id := req.Args[0]
	if err := runtime.Kill(ctx, id, syscall.SIGTERM); err != nil {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: err.Error()}}
	}
	return &Response{ID: req.ID, Result: map[string]any{}}
}

func handleRm(ctx context.Context, req *Request) *Response {
	if len(req.Args) < 1 {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: "rm requires container ID"}}
	}
	id := req.Args[0]
	if err := runtime.Delete(ctx, id); err != nil {
		return &Response{ID: req.ID, Error: &ErrorInfo{Code: 1, Message: err.Error()}}
	}
	return &Response{ID: req.ID, Result: map[string]any{}}
}

func handleSystem(ctx context.Context, req *Request) *Response {
	info := map[string]any{
		"platform": "linux",
		"version":  "1.0",
	}
	return &Response{ID: req.ID, Result: info}
}

func listContainers() ([]map[string]any, error) {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		return nil, err
	}

	var containers []map[string]any
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stateData, err := os.ReadFile(filepath.Join("/run/thrive/containers", entry.Name(), "state.json"))
		if err != nil {
			continue
		}
		var state map[string]any
		if err := json.Unmarshal(stateData, &state); err != nil {
			continue
		}
		containers = append(containers, state)
	}
	return containers, nil
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
}