//go:build darwin

package vm

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// ControlSocketPath returns the host-side Unix socket that CLI subprocesses
// (thrive ps, thrive run, etc.) connect to when the VM is running.
func ControlSocketPath() string {
	return filepath.Join(ThriveDir(), "vm", "control.sock")
}

// ServeControlSocket opens the control socket, accepts connections from other
// thrive processes, and proxies commands to thrived using the provided live bridge.
// Blocks until ctx is cancelled or the bridge closes.
func ServeControlSocket(ctx context.Context, cfg *Config, bridge Bridge) {
	defer bridge.Close()

	sockPath := ControlSocketPath()
	os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Printf("vm: control socket: %v", err)
		return
	}
	defer ln.Close()
	defer os.Remove(sockPath)

	log.Printf("vm: control socket ready — other thrive commands will now work")

	// mu serialises commands to thrived — the vsock bridge is single-stream.
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		go handleControlConn(ctx, conn, bridge, cfg, &mu)
	}
}

// DialControl connects to the control socket and sends one request,
// returning the response bytes. Used by all !linux proxy commands.
func DialControl(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	conn, err := net.DialTimeout("unix", ControlSocketPath(), 5e9)
	if err != nil {
		return nil, fmt.Errorf("thrive desktop start is not running: %w", err)
	}
	defer conn.Close()

	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}
	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	resp, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("control read: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("control parse: %w", err)
	}
	if errInfo, ok := result["error"].(map[string]any); ok {
		return nil, fmt.Errorf("%s", errInfo["message"])
	}
	return json.Marshal(result["result"])
}

// DialControlStream connects to the control socket and streams output lines.
// Used for `thrive logs -f` and `thrive exec`.
func DialControlStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	conn, err := net.DialTimeout("unix", ControlSocketPath(), 5e9)
	if err != nil {
		return fmt.Errorf("thrive desktop start is not running: %w", err)
	}
	defer conn.Close()

	req := map[string]any{"cmd": cmd, "args": args, "opts": opts, "stream": true}
	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	dec := json.NewDecoder(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var resp map[string]any
		if err := dec.Decode(&resp); err != nil {
			return err
		}
		if resp["eof"] == true {
			return nil
		}
		if stream, ok := resp["stream"].(string); ok {
			fmt.Fprintln(out, stream)
		}
	}
}

// handleControlConn processes one CLI request and routes it to thrived.
// If the bridge is broken, it reconnects automatically.
func handleControlConn(ctx context.Context, conn net.Conn, bridge Bridge, cfg *Config, mu *sync.Mutex) {
	defer conn.Close()

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}

	var req map[string]any
	if err := json.Unmarshal(line, &req); err != nil {
		writeControlError(conn, "invalid JSON: "+err.Error())
		return
	}

	cmd, _ := req["cmd"].(string)
	wantStream, _ := req["stream"].(bool)

	// For compose_up: parse embedded YAML spec and sync all referenced images first.
	if cmd == "compose_up" {
		if reqOpts, ok := req["opts"].(map[string]any); ok {
			if spec, ok := reqOpts["spec"].(string); ok && spec != "" {
				mu.Lock()
				syncErr := syncComposeImagesToVM(ctx, bridge, spec)
				mu.Unlock()
				if syncErr != nil {
					writeControlError(conn, "compose image sync failed: "+syncErr.Error())
					return
				}
			}
		}
	}

	// For "run" commands: sync the image to the VM first if it's in the host store
	if cmd == "run" {
		var args []string
		if argsRaw, ok := req["args"].([]any); ok {
			for _, a := range argsRaw {
				if s, ok := a.(string); ok {
					args = append(args, s)
				}
			}
		}
		if len(args) > 0 {
			imageRef := args[0]
			mu.Lock()
			syncErr := syncImageToVM(ctx, bridge, imageRef)
			mu.Unlock()
			if syncErr != nil {
				// Return sync error to user — run with incomplete image would fail later anyway
				writeControlError(conn, "image sync to VM failed: "+syncErr.Error())
				return
			}
		}
	}

	var args []string
	if argsRaw, ok := req["args"].([]any); ok {
		for _, a := range argsRaw {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}
	opts, _ := req["opts"].(map[string]any)

	mu.Lock()
	defer mu.Unlock()

	// Use the shared live bridge; reconnect if it died.
	b, err := ensureBridge(ctx, bridge, cfg)
	if err != nil {
		writeControlError(conn, "VM not reachable: "+err.Error())
		return
	}

	if wantStream {
		b.ExecStream(ctx, cmd, args, opts, conn)
		eof, _ := json.Marshal(map[string]any{"eof": true})
		conn.Write(append(eof, '\n'))
		return
	}

	data, err := b.Exec(ctx, cmd, args, opts)
	if err != nil {
		writeControlError(conn, err.Error())
		return
	}

	// After a successful "run", persist a lightweight container record on the host.
	if cmd == "run" {
		var runResult map[string]any
		if jsonErr := json.Unmarshal(data, &runResult); jsonErr == nil {
			if containerID, ok := runResult["container_id"].(string); ok && containerID != "" {
				imageRef := ""
				if len(args) > 0 {
					imageRef = args[0]
				}
				rec := PersistedContainer{
					ID:        containerID,
					Image:     imageRef,
					Command:   args,
					StartedAt: time.Now(),
					Status:    "running",
				}
				if saveErr := SaveContainerRecord(rec); saveErr != nil {
					log.Printf("vm: warning: persist container record: %v", saveErr)
				}
			}
		}
	}

	var result any
	json.Unmarshal(data, &result)
	resp, _ := json.Marshal(map[string]any{"result": result})
	conn.Write(append(resp, '\n'))
}

// ensureBridge verifies the bridge is alive with a ping. If not, reconnects.
func ensureBridge(ctx context.Context, bridge Bridge, cfg *Config) (Bridge, error) {
	// Quick health check
	_, err := bridge.Exec(ctx, "ping", nil, nil)
	if err == nil {
		return bridge, nil
	}

	// Bridge died — reconnect (thrived is in its reconnect loop)
	log.Printf("vm: bridge unhealthy (%v), reconnecting...", err)
	bridge.Close()

	for i := 0; i < 10; i++ {
		b, dialErr := Dial(ctx, cfg.VMType)
		if dialErr != nil {
			continue
		}
		if _, pingErr := b.Exec(ctx, "ping", nil, nil); pingErr == nil {
			return b, nil
		}
		b.Close()
	}
	return nil, fmt.Errorf("failed to reconnect to VM after 10 attempts")
}

func writeControlError(conn net.Conn, msg string) {
	resp, _ := json.Marshal(map[string]any{"error": map[string]any{"message": msg}})
	conn.Write(append(resp, '\n'))
}

// syncComposeImagesToVM parses a compose YAML string and syncs all referenced images to the VM.
func syncComposeImagesToVM(ctx context.Context, bridge Bridge, composeYAML string) error {
	images := extractImagesFromCompose(composeYAML)
	for _, img := range images {
		log.Printf("vm: compose: syncing image %s...", img)
		if err := syncImageToVM(ctx, bridge, img); err != nil {
			return fmt.Errorf("compose sync %s: %w", img, err)
		}
	}
	return nil
}

// extractImagesFromCompose scans compose YAML for "image: <ref>" lines.
func extractImagesFromCompose(yaml string) []string {
	var images []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(yaml, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "image:") {
			img := strings.TrimSpace(strings.TrimPrefix(trimmed, "image:"))
			img = strings.Trim(img, `"'`)
			if img != "" && !seen[img] {
				seen[img] = true
				images = append(images, img)
			}
		}
	}
	return images
}

// syncImageToVM transfers a locally pulled image to the VM via the vsock bridge.
// Sends layers ONE AT A TIME to avoid 300MB+ single JSON payloads for large images.
func syncImageToVM(ctx context.Context, bridge Bridge, imageRef string) error {
	home, _ := os.UserHomeDir()
	imgDir := filepath.Join(home, ".thrive", "images", image.SafeRef(imageRef))

	manifestData, err := os.ReadFile(filepath.Join(imgDir, "manifest.json"))
	if err != nil {
		return nil // not in host store — skip (VM will try to pull directly)
	}

	var metadata struct {
		Layers []struct {
			Digest string
			Path   string
		}
	}
	if err := json.Unmarshal(manifestData, &metadata); err != nil {
		return fmt.Errorf("syncImageToVM: parse manifest: %w", err)
	}

	log.Printf("vm: syncing image %s (%d layers)...", imageRef, len(metadata.Layers))

	// Step 1: send manifest (no layer data) so thrived knows the image structure
	if _, err := bridge.Exec(ctx, "store-image", []string{imageRef}, map[string]any{
		"manifest": string(manifestData),
		"layers":   []any{},
	}); err != nil {
		return fmt.Errorf("syncImageToVM: store manifest: %w", err)
	}

	// Step 2: send each layer individually — keeps each message small
	for i, l := range metadata.Layers {
		log.Printf("vm: layer %d/%d %s...", i+1, len(metadata.Layers), l.Digest[:12])
		tarGz, err := tarGzipDir(l.Path)
		if err != nil {
			return fmt.Errorf("syncImageToVM: tar layer[%d]: %w", i, err)
		}
		if _, err := bridge.Exec(ctx, "store-layer", []string{imageRef}, map[string]any{
			"digest": l.Digest,
			"data":   encb64.StdEncoding.EncodeToString(tarGz),
		}); err != nil {
			return fmt.Errorf("syncImageToVM: store-layer[%d]: %w", i, err)
		}
	}

	log.Printf("vm: synced %s (%d layers)", imageRef, len(metadata.Layers))
	return nil
}

// tarGzipDir creates a gzipped tar archive of a directory's contents.
func tarGzipDir(dirPath string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == ".done" {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel

		if info.Mode()&os.ModeSymlink != 0 {
			link, _ := os.Readlink(path)
			hdr.Linkname = link
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		}
		return nil
	})

	tw.Close()
	gw.Close()

	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
