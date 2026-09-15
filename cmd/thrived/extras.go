//go:build linux

package main

import (
	"context"
	"io"

	"github.com/thakurprasadrout/thrive/internal/buildx"
	"github.com/thakurprasadrout/thrive/internal/checkpoint"
	"github.com/thakurprasadrout/thrive/internal/plugin"
)

func handleCheckpointCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "checkpoint create requires container and name")
		return
	}
	cp, err := checkpoint.Create(req.Args[0], req.Args[1])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"container": cp.ContainerID, "name": cp.Name}})
}

func handleCheckpointLs(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "checkpoint ls requires a container")
		return
	}
	cps, err := checkpoint.List(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, cp := range cps {
		out = append(out, map[string]any{"name": cp.Name, "created": cp.Created})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"checkpoints": out}})
}

func handleCheckpointRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "checkpoint rm requires container and name")
		return
	}
	if err := checkpoint.Remove(req.Args[0], req.Args[1]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handlePluginInstall(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "plugin install requires name and source")
		return
	}
	p, err := plugin.Install(req.Args[0], req.Args[1])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": p.Name, "enabled": p.Enabled}})
}

func handlePluginEnable(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "plugin enable requires a name")
		return
	}
	if err := plugin.Enable(req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handlePluginDisable(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "plugin disable requires a name")
		return
	}
	if err := plugin.Disable(req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handlePluginInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "plugin inspect requires a name")
		return
	}
	p, err := plugin.Inspect(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"name": p.Name, "enabled": p.Enabled, "created": p.Created,
	}})
}

func handlePluginLs(ctx context.Context, req *Request, w io.Writer) {
	plugins, err := plugin.List()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, p := range plugins {
		out = append(out, map[string]any{"name": p.Name, "enabled": p.Enabled})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"plugins": out}})
}

func handlePluginRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "plugin rm requires a name")
		return
	}
	force, _ := req.Opts["force"].(bool)
	if err := plugin.Remove(req.Args[0], force); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleBuildxDu(ctx context.Context, req *Request, w io.Writer) {
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"size": buildx.CacheUsage("/var/lib/thrive/cache"),
	}})
}

func handleBuildxPrune(ctx context.Context, req *Request, w io.Writer) {
	reclaimed, err := buildx.PruneCache("/var/lib/thrive/cache")
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"reclaimed": reclaimed}})
}
