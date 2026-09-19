//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/swarm"
	"github.com/thakurprasadrout/thrive/pkg/compose"
)

func handleSwarmInit(ctx context.Context, req *Request, w io.Writer) {
	addr, _ := req.Opts["addr"].(string)
	st, err := swarm.Init(addr)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"node": st.NodeID, "addr": st.ManagerAddr, "token": st.WorkerToken,
	}})
}

func handleSwarmJoin(ctx context.Context, req *Request, w io.Writer) {
	addr := ""
	if len(req.Args) > 0 {
		addr = req.Args[0]
	}
	token, _ := req.Opts["token"].(string)
	if err := swarm.Join(addr, token); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleSwarmLeave(ctx context.Context, req *Request, w io.Writer) {
	force, _ := req.Opts["force"].(bool)
	if err := swarm.Leave(force); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleSwarmInspect(ctx context.Context, req *Request, w io.Writer) {
	st, err := swarm.Inspect()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"node": st.NodeID, "manager": st.Manager, "addr": st.ManagerAddr,
	}})
}

func handleSwarmToken(ctx context.Context, req *Request, w io.Writer) {
	role := "worker"
	if len(req.Args) > 0 {
		role = req.Args[0]
	}
	rotate, _ := req.Opts["rotate"].(bool)
	tok, err := swarm.JoinToken(role, rotate)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"token": tok}})
}

func swarmServiceSpec(req *Request, imageRef string) (swarm.ServiceSpec, error) {
	spec := swarm.ServiceSpec{Image: imageRef}
	if v, ok := req.Opts["replicas"].(float64); ok {
		spec.Replicas = int(v)
	}
	if v, ok := req.Opts["env"].([]any); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				spec.Env = append(spec.Env, s)
			}
		}
	}
	if v, ok := req.Opts["network"].(string); ok {
		spec.NetworkMode = v
	}
	if v, ok := req.Opts["ports"].([]any); ok {
		for _, e := range v {
			if pm, ok := e.(map[string]any); ok {
				spec.Ports = append(spec.Ports, runtime.PortMapping{
					HostPort:      intOpt(pm, "host_port"),
					ContainerPort: intOpt(pm, "container_port"),
					Protocol:      stringOpt(pm, "protocol", "tcp"),
				})
			}
		}
	}
	if v, ok := req.Opts["volumes"].([]any); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				src, dst := splitVolume(s)
				spec.Mounts = append(spec.Mounts, runtime.Mount{
					Source: resolveVolumeSource(src), Destination: dst,
					Type: "bind", Options: []string{"rbind"},
				})
			}
		}
	}
	if v, ok := req.Opts["secrets"].([]any); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				spec.Secrets = append(spec.Secrets, s)
			}
		}
	}
	if v, ok := req.Opts["configs"].([]any); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				if i := strings.Index(s, ":"); i > 0 {
					spec.Configs = append(spec.Configs, runtime.ConfigMount{Source: s[:i], Target: s[i+1:]})
				}
			}
		}
	}
	if v, ok := req.Opts["parallelism"].(float64); ok {
		spec.Parallelism = int(v)
	}
	if v, ok := req.Opts["delay"].(float64); ok {
		spec.DelaySecs = int(v)
	}
	if v, ok := req.Opts["restart"].(string); ok {
		spec.Restart = v
	}
	return spec, nil
}

func handleServiceCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service create requires an image")
		return
	}
	name, _ := req.Opts["name"].(string)
	spec, _ := swarmServiceSpec(req, req.Args[0])
	spec.Command = req.Args[1:]
	svc, err := swarm.CreateService(ctx, name, spec)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": svc.Name}})
}

func handleServiceLs(ctx context.Context, req *Request, w io.Writer) {
	services, err := swarm.ListServices()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, svc := range services {
		out = append(out, map[string]any{"name": svc.Name, "image": svc.Spec.Image, "replicas": svc.Spec.Replicas})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"services": out}})
}

func handleServiceInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service inspect requires a name")
		return
	}
	svc, err := swarm.InspectService(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"name": svc.Name, "image": svc.Spec.Image, "replicas": svc.Spec.Replicas,
	}})
}

func handleServicePs(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service ps requires a name")
		return
	}
	svc, err := swarm.InspectService(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	entries, _ := os.ReadDir("/run/thrive/containers")
	prefix := "swarm-" + svc.Name + "-"
	var out []map[string]any
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) <= len(prefix) || e.Name()[:len(prefix)] != prefix {
			continue
		}
		state, err := runtime.State(ctx, e.Name())
		if err != nil {
			continue
		}
		out = append(out, map[string]any{"id": e.Name(), "status": state.Status, "pid": state.PID})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"tasks": out}})
}

func handleServiceRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service rm requires a name")
		return
	}
	if err := swarm.RemoveService(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleServiceScale(ctx context.Context, req *Request, w io.Writer) {
	for _, s := range req.Args {
		name, n, ok := parseScaleArg(s)
		if !ok {
			sendError(w, req.ID, 1, fmt.Sprintf("invalid scale %q", s))
			return
		}
		if err := swarm.ScaleService(ctx, name, n); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleServiceLogs(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service logs requires a name")
		return
	}
	entries, _ := os.ReadDir("/run/thrive/containers")
	prefix := "swarm-" + req.Args[0] + "-"
	var combined string
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) <= len(prefix) || e.Name()[:len(prefix)] != prefix {
			continue
		}
		if data, err := os.ReadFile("/run/thrive/containers/" + e.Name() + "/logs"); err == nil {
			combined += "==> " + e.Name() + " <==\n" + string(data) + "\n"
		}
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"output": combined}})
}

func handleServiceUpdate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service update requires a name")
		return
	}
	newImage, _ := req.Opts["image"].(string)
	if err := swarm.UpdateService(ctx, req.Args[0], newImage); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleServiceRollback(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "service rollback requires a name")
		return
	}
	if err := swarm.RollbackService(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleStackDeploy(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stack deploy requires a name")
		return
	}
	spec, _ := req.Opts["spec"].(string)
	if spec == "" {
		sendError(w, req.ID, 1, "stack deploy: no spec provided")
		return
	}
	f, err := os.CreateTemp("", "thrive-stack-*.yml")
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(f.Name())
	if _, err := io.WriteString(f, spec); err != nil {
		f.Close()
		sendError(w, req.ID, 1, err.Error())
		return
	}
	f.Close()
	cf, err := compose.Load(f.Name())
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	st, err := swarm.DeployStack(ctx, req.Args[0], cf)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"stack": st.Name}})
}

func handleStackLs(ctx context.Context, req *Request, w io.Writer) {
	stacks, err := swarm.ListStacks()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, st := range stacks {
		out = append(out, map[string]any{"name": st.Name, "services": len(st.Services)})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"stacks": out}})
}

func handleStackPs(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stack ps requires a name")
		return
	}
	st, err := swarm.InspectStack(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, svc := range st.Services {
		s, err := swarm.InspectService(svc)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{"service": svc, "image": s.Spec.Image, "replicas": s.Spec.Replicas})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"services": out}})
}

func handleStackServices(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stack services requires a name")
		return
	}
	st, err := swarm.InspectStack(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []any
	for _, svc := range st.Services {
		out = append(out, svc)
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"services": out}})
}

func handleStackRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stack rm requires a name")
		return
	}
	if err := swarm.RemoveStack(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func intOpt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func stringOpt(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func parseScaleArg(s string) (string, int, bool) {	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			var n int
			if _, err := fmt.Sscanf(s[i+1:], "%d", &n); err != nil || n < 0 {
				return "", 0, false
			}
			return s[:i], n, true
		}
	}
	return "", 0, false
}
