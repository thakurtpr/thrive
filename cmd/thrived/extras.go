//go:build linux

package main

import (
	"context"
	"io"
	"os"
	"runtime"

	"github.com/thakurprasadrout/thrive/internal/buildx"
	"github.com/thakurprasadrout/thrive/internal/checkpoint"
	"github.com/thakurprasadrout/thrive/internal/plugin"
	thrruntime "github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/swarm"
	"github.com/thakurprasadrout/thrive/internal/swarmconfig"
	thrsystem "github.com/thakurprasadrout/thrive/internal/system"
)

// thrivedVersion is overridden at release time via ldflags.
var thrivedVersion = "dev"

func handleVersion(ctx context.Context, req *Request, w io.Writer) {
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"engine": "thrive (daemonless)", "version": thrivedVersion,
		"os": runtime.GOOS, "arch": runtime.GOARCH,
	}})
}

func handleAttach(ctx context.Context, req *Request, w io.Writer) {
	// Attach streams the log tail; stdin forwarding is client-side exec.
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "attach requires a container")
		return
	}
	logReq := &Request{ID: req.ID, Cmd: "logs", Args: req.Args, Opts: map[string]any{"follow": true}}
	handleLogs(ctx, logReq, w)
}

func handleSwarmNodeLs(ctx context.Context, req *Request, w io.Writer) {
	st, err := swarm.Inspect()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"nodes": []map[string]any{{"id": st.NodeID, "role": "manager", "addr": st.ManagerAddr}},
	}})
}

func handleSwarmNodeInspect(ctx context.Context, req *Request, w io.Writer) {
	st, err := swarm.Inspect()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"id": st.NodeID, "role": "manager", "addr": st.ManagerAddr,
	}})
}

func handleSwarmNodePromote(ctx context.Context, req *Request, w io.Writer) {
	if err := swarm.RequireInitialised(); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"role": "manager"}})
}

func handleSwarmNodeDemote(ctx context.Context, req *Request, w io.Writer) {
	sendError(w, req.ID, 1, "node: cannot demote the last manager (single-node swarm)")
}

func handleSwarmNodePs(ctx context.Context, req *Request, w io.Writer) {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		state, err := thrruntime.State(ctx, e.Name())
		if err != nil {
			continue
		}
		out = append(out, map[string]any{"id": e.Name(), "status": state.Status})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"tasks": out}})
}

func handleConfigCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "config create requires a name")
		return
	}
	data, _ := req.Opts["data"].(string)
	cfg, err := swarmconfig.Create(req.Args[0], []byte(data))
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": cfg.Name, "size": cfg.Size}})
}

func handleConfigLs(ctx context.Context, req *Request, w io.Writer) {
	cfgs, err := swarmconfig.List()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, c := range cfgs {
		out = append(out, map[string]any{"name": c.Name, "size": c.Size})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"configs": out}})
}

func handleConfigInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "config inspect requires a name")
		return
	}
	cfg, err := swarmconfig.Inspect(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": cfg.Name, "size": cfg.Size}})
}

func handleConfigRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "config rm requires a name")
		return
	}
	if err := swarmconfig.Remove(req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleImagePrune(ctx context.Context, req *Request, w io.Writer) {
	all, _ := req.Opts["all"].(bool)
	if !all {
		sendError(w, req.ID, 1, "image prune: nothing to do without --all (all local images carry refs)")
		return
	}
	deleted, reclaimed, err := thrsystem.PruneImages()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"deleted": deleted, "reclaimed": reclaimed}})
}

func handleContainerPrune(ctx context.Context, req *Request, w io.Writer) {
	deleted, reclaimed, err := thrsystem.PruneContainers()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"deleted": deleted, "reclaimed": reclaimed}})
}

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
	var p *plugin.Plugin
	var err error
	if _, statErr := os.Stat(req.Args[1]); statErr == nil {
		p, err = plugin.Install(req.Args[0], req.Args[1])
	} else {
		p, err = plugin.InstallFromRef(ctx, req.Args[0], req.Args[1])
	}
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
