//go:build linux

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func base64Decode(s string) ([]byte, error) {
	return encb64.StdEncoding.DecodeString(s)
}

func extractGzipTar(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		n := filepath.Clean(hdr.Name)
		if strings.HasPrefix(n, "..") {
			continue
		}
		target := filepath.Join(destDir, n)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(hdr.Mode))
		case tar.TypeReg, tar.TypeRegA:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Symlink(hdr.Linkname, target)
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Link(linkTarget, target)
		}
	}
	return nil
}

func dispatch(ctx context.Context, req *Request, w io.Writer) {
	switch req.Cmd {
	case "store-image":
		handleStoreImage(ctx, req, w)
	case "store-layer":
		handleStoreLayer(ctx, req, w)
	case "debug-ls":
		handleDebugLs(ctx, req, w)
	case "ps":
		handlePS(ctx, req, w)
	case "run":
		handleRun(ctx, req, w)
	case "pull":
		handlePull(ctx, req, w)
	case "images":
		handleImages(ctx, req, w)
	case "logs":
		handleLogs(ctx, req, w)
	case "exec":
		handleExec(ctx, req, w)
	case "kill":
		handleKill(ctx, req, w)
	case "stop":
		handleStop(ctx, req, w)
	case "start":
		handleStart(ctx, req, w)
	case "restart":
		handleRestart(ctx, req, w)
	case "rm":
		handleRm(ctx, req, w)
	case "rmi":
		handleRmi(ctx, req, w)
	case "inspect":
		handleInspect(ctx, req, w)
	case "system":
		handleSystem(ctx, req, w)
	case "cp":
		handleCp(ctx, req, w)
	case "ping":
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ok": true}})
	default:
		sendError(w, req.ID, 1, "unknown command: "+req.Cmd)
	}
}

func handlePS(ctx context.Context, req *Request, w io.Writer) {
	containers, err := listContainers()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{
		ID:     req.ID,
		Result: map[string]any{"containers": containers},
	})
}

func handleStoreImage(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "store-image requires image ref")
		return
	}
	ref := req.Args[0]
	imgDir := "/var/lib/thrive/images/" + image.SafeRef(ref)

	// Write manifest.json
	manifest, _ := req.Opts["manifest"].(string)
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("store-image mkdir: %v", err))
		return
	}
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("store-image write manifest: %v", err))
		return
	}

	// Extract layers
	layersRaw, _ := req.Opts["layers"].([]any)
	for _, l := range layersRaw {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		digest, _ := lm["digest"].(string)
		b64data, _ := lm["data"].(string)

		layerDir := filepath.Join(imgDir, "layers", digest)
		doneMarker := layerDir + "/.done"
		if _, err := os.Stat(doneMarker); err == nil {
			continue // already extracted
		}

		if err := os.MkdirAll(layerDir, 0755); err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("store-image mkdir layer: %v", err))
			return
		}

		tarGzData, err := base64Decode(b64data)
		if err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("store-image decode: %v", err))
			return
		}

		if err := extractGzipTar(tarGzData, layerDir); err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("store-image extract: %v", err))
			return
		}
		os.WriteFile(doneMarker, []byte("done"), 0644)
	}

	// Rewrite manifest.json Layer.Path to VM-local paths (original points to macOS host)
	var meta map[string]any
	if err := json.Unmarshal([]byte(manifest), &meta); err == nil {
		if layerList, ok := meta["Layers"].([]any); ok {
			for i, l := range layerList {
				if lm, ok := l.(map[string]any); ok {
					if digest, ok := lm["Digest"].(string); ok {
						lm["Path"] = filepath.Join(imgDir, "layers", digest)
						layerList[i] = lm
					}
				}
			}
			meta["Layers"] = layerList
		}
		if updated, err := json.Marshal(meta); err == nil {
			os.WriteFile(filepath.Join(imgDir, "manifest.json"), updated, 0644)
		}
	}

	log.Printf("thrived: stored image %s", ref)
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ok": true, "ref": ref}})
}

func handleDebugLs(ctx context.Context, req *Request, w io.Writer) {
	path := "/var/lib/thrive/images"
	if len(req.Args) > 0 {
		path = req.Args[0]
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"error": err.Error(), "path": path}})
		return
	}
	var names []string
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		names = append(names, fmt.Sprintf("%s (%d)", e.Name(), size))
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"path": path, "entries": names}})
}

func handleStoreLayer(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "store-layer requires image ref")
		return
	}
	ref := req.Args[0]
	digest, _ := req.Opts["digest"].(string)
	b64data, _ := req.Opts["data"].(string)

	imgDir := "/var/lib/thrive/images/" + image.SafeRef(ref)
	layerDir := filepath.Join(imgDir, "layers", digest)
	doneMarker := layerDir + "/.done"

	// Always re-extract: previous broken syncs may have left a .done marker
	// without complete layer contents (e.g., from failed large-payload messages).
	os.Remove(doneMarker)

	if err := os.MkdirAll(layerDir, 0755); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("store-layer mkdir: %v", err))
		return
	}

	tarGzData, err := base64Decode(b64data)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("store-layer decode: %v", err))
		return
	}

	if err := extractGzipTar(tarGzData, layerDir); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("store-layer extract: %v", err))
		return
	}

	os.WriteFile(doneMarker, []byte("done"), 0644)
	log.Printf("thrived: stored layer %s for %s", digest[:12], ref)
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ok": true}})
}

func handlePull(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "pull requires image reference")
		return
	}
	ref := req.Args[0]
	log.Printf("thrived: pulling %s", ref)
	img, err := image.Pull(ctx, ref, image.PullOptions{})
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("pull %s: %v", ref, err))
		return
	}
	writeResponse(w, &Response{
		ID: req.ID,
		Result: map[string]any{
			"ref":    img.Ref,
			"digest": img.Digest,
			"layers": len(img.Layers),
		},
	})
}

func handleImages(ctx context.Context, req *Request, w io.Writer) {
	imgs, err := image.List(ctx)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var result []map[string]any
	for _, img := range imgs {
		digest := img.Digest
		if len(digest) > 12 {
			digest = digest[:12]
		}
		result = append(result, map[string]any{
			"ref":    img.Ref,
			"digest": digest,
			"layers": len(img.Layers),
		})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"images": result}})
}

func handleRun(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "run requires image argument")
		return
	}

	imageRef := req.Args[0]
	cmd := req.Args[1:]

	// Images are pulled on the macOS host and shared via virtiofs.
	// Check if the image is locally available before attempting a network pull.
	imgPath := "/var/lib/thrive/images/" + image.SafeRef(imageRef) + "/manifest.json"
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		log.Printf("thrived: image not found locally, attempting pull: %s", imageRef)
		if _, pullErr := image.Pull(ctx, imageRef, image.PullOptions{}); pullErr != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("image %s not found — run `thrive pull %s` first on your Mac: %v", imageRef, imageRef, pullErr))
			return
		}
	}

	var ports []runtime.PortMapping
	if portsRaw, ok := req.Opts["ports"].([]any); ok {
		for _, p := range portsRaw {
			if pm, ok := p.(map[string]any); ok {
				ports = append(ports, runtime.PortMapping{
					HostPort:      int(pm["host_port"].(float64)),
					ContainerPort: int(pm["container_port"].(float64)),
					Protocol:      stringOrDefault(pm["protocol"], "tcp"),
				})
			}
		}
	}

	var mounts []runtime.Mount
	if mountsRaw, ok := req.Opts["volumes"].([]any); ok {
		for _, m := range mountsRaw {
			if ms, ok := m.(string); ok {
				src, dst := splitVolume(ms)
				mounts = append(mounts, runtime.Mount{
					Source:      src,
					Destination: dst,
					Type:        "bind",
					Options:     []string{"rbind"},
				})
			}
		}
	}

	var envVars []string
	if envRaw, ok := req.Opts["env"].([]any); ok {
		for _, e := range envRaw {
			if s, ok := e.(string); ok {
				envVars = append(envVars, s)
			}
		}
	}

	name, _ := req.Opts["name"].(string)
	containerID := name
	if containerID == "" {
		containerID = generateID()
	}

	cfg := runtime.ContainerConfig{
		ID:      containerID,
		Image:   imageRef,
		Command: cmd,
		Env:     envVars,
		Ports:   ports,
		Mounts:  mounts,
	}

	if _, err := runtime.Create(ctx, cfg); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}

	detach, _ := req.Opts["detach"].(bool)
	if detach {
		go func() {
			if err := runtime.Start(ctx, cfg.ID); err != nil {
				log.Printf("thrived: container start error: %v", err)
			}
		}()
	} else {
		if err := runtime.Start(ctx, cfg.ID); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
	}

	writeResponse(w, &Response{
		ID:     req.ID,
		Result: map[string]any{"container_id": cfg.ID},
	})
}

func handleLogs(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "logs requires container ID")
		return
	}
	containerID := req.Args[0]
	follow, _ := req.Opts["follow"].(bool)

	logPath := filepath.Join("/run/thrive/containers", containerID, "logs")
	f, err := os.Open(logPath)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("cannot open log file: %v", err))
		return
	}
	defer f.Close()

	if !follow {
		// Single response — prevents stale EOF messages corrupting the bridge buffer
		data, _ := io.ReadAll(f)
		writeResponse(w, &Response{
			ID:     req.ID,
			Result: map[string]any{"output": string(data)},
		})
		return
	}

	// Follow mode: stream lines (caller uses ExecStream)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		writeResponse(w, &Response{ID: req.ID, Stream: scanner.Text()})
	}
	for {
		select {
		case <-ctx.Done():
			writeResponse(w, &Response{ID: req.ID, EOF: true})
			return
		default:
		}
		if scanner.Scan() {
			writeResponse(w, &Response{ID: req.ID, Stream: scanner.Text()})
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func handleExec(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "exec requires container ID and command")
		return
	}
	containerID := req.Args[0]
	cmd := req.Args[1:]

	state, err := runtime.State(ctx, containerID)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}
	if state.PID == 0 || state.Status != "running" {
		sendError(w, req.ID, 1, "container is not running")
		return
	}

	nsenterArgs := []string{
		"--target", strconv.Itoa(state.PID),
		"--mount", "--pid", "--ipc", "--uts", "--net",
		"--",
	}
	nsenterArgs = append(nsenterArgs, cmd...)

	execCmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	pr, pw := io.Pipe()
	execCmd.Stdout = pw
	execCmd.Stderr = pw

	if err := execCmd.Start(); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("nsenter failed: %v", err))
		return
	}

	go func() {
		execCmd.Wait()
		pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		writeResponse(w, &Response{ID: req.ID, Stream: scanner.Text()})
	}
	pr.Close()

	exitCode := 0
	if execCmd.ProcessState != nil {
		exitCode = execCmd.ProcessState.ExitCode()
	}
	writeResponse(w, &Response{
		ID:     req.ID,
		EOF:    true,
		Result: map[string]any{"exit_code": exitCode},
	})
}

func handleKill(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "kill requires container ID")
		return
	}
	if err := runtime.Kill(ctx, req.Args[0], syscall.SIGKILL); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleStop(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stop requires container ID")
		return
	}
	id := req.Args[0]

	state, err := runtime.State(ctx, id)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}
	if state.Status != "running" {
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
		return
	}

	_ = runtime.Kill(ctx, id, syscall.SIGTERM)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		s, err := runtime.State(ctx, id)
		if err != nil || s.Status == "stopped" {
			writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
			return
		}
	}

	_ = runtime.Kill(ctx, id, syscall.SIGKILL)
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleStart(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "start requires container ID")
		return
	}
	id := req.Args[0]

	containerDir := filepath.Join("/run/thrive/containers", id)
	stateData, err := os.ReadFile(filepath.Join(containerDir, "state.json"))
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}
	var state runtime.ContainerState
	if err := json.Unmarshal(stateData, &state); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	if state.Status == "running" {
		sendError(w, req.ID, 1, "container already running")
		return
	}
	state.Status = "created"
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(containerDir, "state.json"), data, 0644)

	if err := runtime.Start(ctx, id); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleRestart(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "restart requires container ID")
		return
	}
	id := req.Args[0]

	stopReq := &Request{ID: req.ID, Cmd: "stop", Args: []string{id}}
	handleStop(ctx, stopReq, discardWriter{})

	startReq := &Request{ID: req.ID, Cmd: "start", Args: []string{id}}
	handleStart(ctx, startReq, w)
}

func handleRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "rm requires container ID")
		return
	}
	if err := runtime.Delete(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleRmi(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "rmi requires image ref")
		return
	}
	if err := image.Remove(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"removed": req.Args[0]}})
}

func handleInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "inspect requires container ID")
		return
	}
	id := req.Args[0]

	state, err := runtime.State(ctx, id)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}

	configPath := filepath.Join("/run/thrive/containers", id, "config.json")
	configData, _ := os.ReadFile(configPath)
	var cfg map[string]any
	json.Unmarshal(configData, &cfg)

	writeResponse(w, &Response{
		ID: req.ID,
		Result: map[string]any{
			"id":     state.ID,
			"status": state.Status,
			"pid":    state.PID,
			"config": cfg,
		},
	})
}

func handleSystem(ctx context.Context, req *Request, w io.Writer) {
	writeResponse(w, &Response{
		ID: req.ID,
		Result: map[string]any{
			"platform":   "linux",
			"version":    "1.0",
			"runtime":    "thrive",
			"rootless":   true,
			"daemonless": true,
		},
	})
}

func listContainers() ([]map[string]any, error) {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
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

func splitVolume(v string) (src, dst string) {
	for i, c := range v {
		if c == ':' {
			return v[:i], v[i+1:]
		}
	}
	return v, v
}

func stringOrDefault(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func handleCp(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "cp requires container ID")
		return
	}
	containerID := req.Args[0]
	direction, _ := req.Opts["direction"].(string)

	mergedDir := filepath.Join("/run/thrive/containers", containerID, "merged")
	if _, err := os.Stat(mergedDir); os.IsNotExist(err) {
		upperDir := filepath.Join("/run/thrive/containers", containerID, "upper")
		if _, err2 := os.Stat(upperDir); os.IsNotExist(err2) {
			sendError(w, req.ID, 1, fmt.Sprintf("container %s: not found or not started", containerID))
			return
		}
		mergedDir = upperDir
	}

	switch direction {
	case "from":
		srcPath, _ := req.Opts["src_path"].(string)
		data, err := os.ReadFile(filepath.Join(mergedDir, srcPath))
		if err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("cp from: %v", err))
			return
		}
		writeResponse(w, &Response{
			ID:     req.ID,
			Result: map[string]any{"data": encb64.StdEncoding.EncodeToString(data)},
		})
	case "to":
		dstPath, _ := req.Opts["dst_path"].(string)
		encoded, _ := req.Opts["data"].(string)
		fileData, err := base64Decode(encoded)
		if err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("cp to: decode: %v", err))
			return
		}
		target := filepath.Join(mergedDir, dstPath)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("cp to: mkdir: %v", err))
			return
		}
		if err := os.WriteFile(target, fileData, 0644); err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("cp to: write: %v", err))
			return
		}
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ok": true}})
	default:
		sendError(w, req.ID, 1, "cp: direction must be 'from' or 'to'")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
