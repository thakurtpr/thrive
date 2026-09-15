//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thakurprasadrout/thrive/pkg/compose"
)

func composeProject(req *Request) string {
	if p, _ := req.Opts["project"].(string); p != "" {
		return p
	}
	if f, _ := req.Opts["file"].(string); f != "" {
		if abs, err := filepath.Abs(filepath.Dir(f)); err == nil {
			return filepath.Base(abs)
		}
	}
	return "default"
}

func composeTargets(req *Request) []string {
	var out []string
	if raw, ok := req.Opts["services"].([]any); ok {
		for _, s := range raw {
			if name, ok := s.(string); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

// loadComposeSpec materialises the proxied compose YAML into a temp file.
func loadComposeSpec(req *Request) (*compose.ComposeFile, string, error) {
	spec, _ := req.Opts["spec"].(string)
	if spec == "" {
		return nil, "", fmt.Errorf("compose: no spec provided")
	}
	f, err := os.CreateTemp("", "thrive-compose-*.yml")
	if err != nil {
		return nil, "", err
	}
	if _, err := io.WriteString(f, spec); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, "", err
	}
	_ = f.Close()
	cf, err := compose.Load(f.Name())
	if err != nil {
		_ = os.Remove(f.Name())
		return nil, "", err
	}
	return cf, f.Name(), nil
}

func handleComposeUp(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	scales := map[string]int{}
	if raw, ok := req.Opts["scale"].([]any); ok {
		for _, s := range raw {
			if str, ok := s.(string); ok {
				if i := strings.Index(str, "="); i > 0 {
					if n, err := strconv.Atoi(str[i+1:]); err == nil {
						scales[str[:i]] = n
					}
				}
			}
		}
	}
	if err := compose.UpScaled(ctx, cf, composeProject(req), scales); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleComposeDown(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	if err := compose.Down(ctx, cf, composeProject(req)); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleComposePs(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	statuses, err := compose.Ps(ctx, cf, composeProject(req))
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, s := range statuses {
		out = append(out, map[string]any{"service": s.Name, "id": s.ID, "status": s.Status})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"containers": out}})
}

func handleComposeLogs(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	targets := composeTargets(req)
	if len(targets) == 0 {
		for name := range cf.Services {
			targets = append(targets, name)
		}
	}
	var combined []string
	for _, name := range targets {
		statuses, _ := compose.Ps(ctx, &compose.ComposeFile{Services: map[string]*compose.ServiceDef{name: cf.Services[name]}}, composeProject(req))
		for _, s := range statuses {
			data, err := os.ReadFile(filepath.Join("/run/thrive/containers", s.ID, "logs"))
			if err == nil {
				combined = append(combined, fmt.Sprintf("==> %s <==\n%s", name, string(data)))
			}
		}
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"output": strings.Join(combined, "\n")}})
}

func handleComposePull(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	if err := compose.Pull(ctx, cf, composeTargets(req)); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleComposeConfig(ctx context.Context, req *Request, w io.Writer) {
	cf, tmp, err := loadComposeSpec(req)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	defer os.Remove(tmp) //nolint:errcheck

	out, err := compose.Config(cf)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	// Print server-side for thrived logs; proxy ignores output content.
	fmt.Println(out)
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"config": out}})
}
