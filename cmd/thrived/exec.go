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

	"github.com/thakurprasadrout/thrive/internal/checkpoint"
	"github.com/thakurprasadrout/thrive/internal/events"
	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/registry"
	"github.com/thakurprasadrout/thrive/internal/runtime"
	thrsystem "github.com/thakurprasadrout/thrive/internal/system"
	"github.com/thakurprasadrout/thrive/internal/volume"
)

// checkpointRestoreBridge restores a container from a checkpoint.
func checkpointRestoreBridge(ctx context.Context, id, name string) error {
	return checkpoint.Restore(ctx, id, name)
}

// resolveVolumeSource maps a named volume to its host path, creating it
// on demand. Bind paths pass through unchanged.
func resolveVolumeSource(src string) string {
	if volume.IsNamedVolume(src) {
		if vol, err := volume.Ensure(src); err == nil {
			return vol.Path
		}
	}
	return src
}

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
	case "create":
		handleCreate(ctx, req, w)
	case "pause":
		handlePause(ctx, req, w)
	case "unpause":
		handleUnpause(ctx, req, w)
	case "wait":
		handleWait(ctx, req, w)
	case "rename":
		handleRename(ctx, req, w)
	case "stats":
		handleStats(ctx, req, w)
	case "update":
		handleUpdate(ctx, req, w)
	case "top":
		handleTop(ctx, req, w)
	case "port":
		handlePort(ctx, req, w)
	case "diff":
		handleDiff(ctx, req, w)
	case "export":
		handleExport(ctx, req, w)
	case "commit":
		handleCommit(ctx, req, w)
	case "save":
		handleSave(ctx, req, w)
	case "load":
		handleLoad(ctx, req, w)
	case "import":
		handleImport(ctx, req, w)
	case "history":
		handleHistory(ctx, req, w)
	case "network-create":
		handleNetworkCreate(ctx, req, w)
	case "network-ls":
		handleNetworkLs(ctx, req, w)
	case "network-inspect":
		handleNetworkInspect(ctx, req, w)
	case "network-rm":
		handleNetworkRm(ctx, req, w)
	case "network-connect":
		handleNetworkConnect(ctx, req, w)
	case "network-disconnect":
		handleNetworkDisconnect(ctx, req, w)
	case "network-prune":
		handleNetworkPrune(ctx, req, w)
	case "volume-create":
		handleVolumeCreate(ctx, req, w)
	case "volume-ls":
		handleVolumeLs(ctx, req, w)
	case "volume-inspect":
		handleVolumeInspect(ctx, req, w)
	case "volume-rm":
		handleVolumeRm(ctx, req, w)
	case "volume-prune":
		handleVolumePrune(ctx, req, w)
	case "system-df":
		handleSystemDf(ctx, req, w)
	case "system-events":
		handleSystemEvents(ctx, req, w)
	case "system-prune":
		handleSystemPrune(ctx, req, w)
	case "compose_up":
		handleComposeUp(ctx, req, w)
	case "compose_down":
		handleComposeDown(ctx, req, w)
	case "compose_ps":
		handleComposePs(ctx, req, w)
	case "compose_logs":
		handleComposeLogs(ctx, req, w)
	case "compose_pull":
		handleComposePull(ctx, req, w)
	case "compose_config":
		handleComposeConfig(ctx, req, w)
	case "swarm-init":
		handleSwarmInit(ctx, req, w)
	case "swarm-join":
		handleSwarmJoin(ctx, req, w)
	case "swarm-leave":
		handleSwarmLeave(ctx, req, w)
	case "swarm-inspect":
		handleSwarmInspect(ctx, req, w)
	case "swarm-token":
		handleSwarmToken(ctx, req, w)
	case "service-create":
		handleServiceCreate(ctx, req, w)
	case "service-ls":
		handleServiceLs(ctx, req, w)
	case "service-inspect":
		handleServiceInspect(ctx, req, w)
	case "service-ps":
		handleServicePs(ctx, req, w)
	case "service-rm":
		handleServiceRm(ctx, req, w)
	case "service-scale":
		handleServiceScale(ctx, req, w)
	case "service-logs":
		handleServiceLogs(ctx, req, w)
	case "service-update":
		handleServiceUpdate(ctx, req, w)
	case "service-rollback":
		handleServiceRollback(ctx, req, w)
	case "stack-deploy":
		handleStackDeploy(ctx, req, w)
	case "stack-ls":
		handleStackLs(ctx, req, w)
	case "stack-ps":
		handleStackPs(ctx, req, w)
	case "stack-services":
		handleStackServices(ctx, req, w)
	case "stack-rm":
		handleStackRm(ctx, req, w)
	case "checkpoint-create":
		handleCheckpointCreate(ctx, req, w)
	case "checkpoint-ls":
		handleCheckpointLs(ctx, req, w)
	case "checkpoint-rm":
		handleCheckpointRm(ctx, req, w)
	case "plugin-install":
		handlePluginInstall(ctx, req, w)
	case "plugin-enable":
		handlePluginEnable(ctx, req, w)
	case "plugin-disable":
		handlePluginDisable(ctx, req, w)
	case "plugin-inspect":
		handlePluginInspect(ctx, req, w)
	case "plugin-ls":
		handlePluginLs(ctx, req, w)
	case "plugin-rm":
		handlePluginRm(ctx, req, w)
	case "buildx-du":
		handleBuildxDu(ctx, req, w)
	case "buildx-prune":
		handleBuildxPrune(ctx, req, w)
	case "version":
		handleVersion(ctx, req, w)
	case "attach":
		handleAttach(ctx, req, w)
	case "swarm-node-ls":
		handleSwarmNodeLs(ctx, req, w)
	case "swarm-node-inspect":
		handleSwarmNodeInspect(ctx, req, w)
	case "swarm-node-promote":
		handleSwarmNodePromote(ctx, req, w)
	case "swarm-node-demote":
		handleSwarmNodeDemote(ctx, req, w)
	case "swarm-node-ps":
		handleSwarmNodePs(ctx, req, w)
	case "config-create":
		handleConfigCreate(ctx, req, w)
	case "config-ls":
		handleConfigLs(ctx, req, w)
	case "config-inspect":
		handleConfigInspect(ctx, req, w)
	case "config-rm":
		handleConfigRm(ctx, req, w)
	case "image-prune":
		handleImagePrune(ctx, req, w)
	case "container-prune":
		handleContainerPrune(ctx, req, w)
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
	// Cosign verification is host-side: the PEM key file lives on the
	// client, not in the VM. The Windows/macOS clients verify after a
	// successful pull and remove the image on failure. Reject explicitly
	// so a future client never silently skips verification.
	if v, _ := req.Opts["verify"].(bool); v {
		sendError(w, req.ID, 1, "pull: --verify is handled host-side by the thrive client (key file lives on the host), not by thrived")
		return
	}
	username, _ := req.Opts["username"].(string)
	password, _ := req.Opts["password"].(string)
	platform, _ := req.Opts["platform"].(string)
	if platform != "" {
		if _, err := image.ParsePlatform(platform); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
	}
	pullOpts := image.PullOptions{Username: username, Password: password, Platform: platform}
	if allTags, _ := req.Opts["all_tags"].(bool); allTags {
		tags, err := registry.ListRepoTags(ctx, ref, username, password)
		if err != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("pull %s: list tags: %v", ref, err))
			return
		}
		pulled := 0
		for _, r := range tags {
			if _, err := image.Pull(ctx, r, pullOpts); err != nil {
				sendError(w, req.ID, 1, fmt.Sprintf("pull %s: %v", r, err))
				return
			}
			pulled++
		}
		writeResponse(w, &Response{
			ID: req.ID,
			Result: map[string]any{
				"ref": ref, "pulled": pulled,
			},
		})
		return
	}
	img, err := image.Pull(ctx, ref, pullOpts)
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
				src = resolveVolumeSource(src)
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

	var secretNames []string
	if secretsRaw, ok := req.Opts["secrets"].([]any); ok {
		for _, e := range secretsRaw {
			if s, ok := e.(string); ok {
				secretNames = append(secretNames, s)
			}
		}
	}

	var configMounts []runtime.ConfigMount
	if configsRaw, ok := req.Opts["configs"].([]any); ok {
		for _, e := range configsRaw {
			if s, ok := e.(string); ok {
				if i := strings.Index(s, ":"); i > 0 {
					configMounts = append(configMounts, runtime.ConfigMount{Source: s[:i], Target: s[i+1:]})
				}
			}
		}
	}

	netMode, _ := req.Opts["network"].(string)
	cfg := runtime.ContainerConfig{
		ID:          containerID,
		Image:       imageRef,
		Command:     cmd,
		Env:         envVars,
		Secrets:     secretNames,
		Configs:     configMounts,
		Ports:       ports,
		Mounts:      mounts,
		NetworkMode: netMode,
		Resources:   resourceLimitsFromOpts(req.Opts),
	}
	if restartSpec, _ := req.Opts["restart"].(string); restartSpec != "" {
		if policy, err := runtime.ParseRestartPolicy(restartSpec); err == nil {
			cfg.RestartPolicy = policy
		}
	}

	if _, err := runtime.Create(ctx, cfg); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}

	detach, _ := req.Opts["detach"].(bool)
	if detach {
		go func() {
			if _, err := runtime.Start(ctx, cfg.ID); err != nil {
				log.Printf("thrived: container start error: %v", err)
			}
		}()
	} else {
		if _, err := runtime.Start(ctx, cfg.ID); err != nil {
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
	// Daemonless log files carry no per-line timestamps; refuse time
	// filters explicitly rather than returning misleading subsets.
	if since, _ := req.Opts["since"].(string); since != "" {
		sendError(w, req.ID, 1, "logs: --since requires per-line timestamps, which thrive's daemonless log files do not record")
		return
	}
	if until, _ := req.Opts["until"].(string); until != "" {
		sendError(w, req.ID, 1, "logs: --until requires per-line timestamps, which thrive's daemonless log files do not record")
		return
	}
	if ts, _ := req.Opts["timestamps"].(bool); ts {
		sendError(w, req.ID, 1, "logs: --timestamps requires per-line timestamps, which thrive's daemonless log files do not record")
		return
	}

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
		if n, ok := tailCount(req); ok {
			data = lastLines(data, n)
		}
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

	// Optional env/workdir (docker exec -e/-w parity). Env uses the env(1)
	// prefix; workdir uses a sh cd+exec wrapper (no nsenter version dependency).
	var envVars []string
	if envRaw, ok := req.Opts["env"].([]any); ok {
		for _, e := range envRaw {
			if s, ok := e.(string); ok {
				envVars = append(envVars, s)
			}
		}
	}
	if len(envVars) > 0 {
		cmd = append(append([]string{"env"}, envVars...), cmd...)
	}
	if workdir, _ := req.Opts["workdir"].(string); workdir != "" {
		script := "cd '" + strings.ReplaceAll(workdir, "'", "'\"'\"'") + "' && exec \"$@\""
		cmd = append([]string{"/bin/sh", "-c", script, "thrive-exec"}, cmd...)
	}

	var execCmd *exec.Cmd
	if state.HasNamespaces {
		// Container was started with namespace isolation — enter namespaces via nsenter.
		nsenterArgs := []string{
			"--target", strconv.Itoa(state.PID),
			"--mount", "--pid", "--ipc", "--uts", "--net",
			"--",
		}
		nsenterArgs = append(nsenterArgs, cmd...)
		execCmd = exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	} else {
		// Container runs chroot-only (e.g. inside VM). Exec directly in the rootfs.
		rootfsDir := filepath.Join("/run/thrive/containers", containerID, "merged")
		if _, statErr := os.Stat(rootfsDir); os.IsNotExist(statErr) && state.RootfsPath != "" {
			rootfsDir = state.RootfsPath
		}
		binary := cmd[0]
		// Resolve relative binary names through standard PATH dirs inside the rootfs.
		if !strings.HasPrefix(binary, "/") {
			for _, dir := range []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"} {
				if _, statErr := os.Stat(filepath.Join(rootfsDir, dir, binary)); statErr == nil {
					binary = filepath.Join(dir, binary)
					break
				}
			}
		}
		execCmd = exec.CommandContext(ctx, binary, cmd[1:]...)
		execCmd.SysProcAttr = &syscall.SysProcAttr{Chroot: rootfsDir}
		execCmd.Dir = "/"
	}

	pr, pw := io.Pipe()
	execCmd.Stdout = pw
	execCmd.Stderr = pw

	if err := execCmd.Start(); err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("exec failed: %v", err))
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

	if checkpointName, _ := req.Opts["checkpoint"].(string); checkpointName != "" {
		if err := checkpointRestoreBridge(ctx, id, checkpointName); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
		return
	}

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

	if _, err := runtime.Start(ctx, id); err != nil {
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
		sendError(w, req.ID, 1, "inspect requires a container or image name")
		return
	}
	id := req.Args[0]

	if state, err := runtime.State(ctx, id); err == nil {
		configPath := filepath.Join("/run/thrive/containers", id, "config.json")
		configData, _ := os.ReadFile(configPath)
		var cfg map[string]any
		json.Unmarshal(configData, &cfg)

		writeResponse(w, &Response{
			ID: req.ID,
			Result: map[string]any{
				"type":   "container",
				"id":     state.ID,
				"status": state.Status,
				"pid":    state.PID,
				"config": cfg,
			},
		})
		return
	}

	if imgs, err := image.List(ctx); err == nil {
		for _, img := range imgs {
			if img.Ref == id || img.Digest == id {
				var layers []map[string]any
				for _, l := range img.Layers {
					layers = append(layers, map[string]any{"digest": l.Digest, "size": l.Size})
				}
				writeResponse(w, &Response{
					ID: req.ID,
					Result: map[string]any{
						"type": "image", "ref": img.Ref,
						"digest": img.Digest, "layers": layers,
					},
				})
				return
			}
		}
	}
	sendError(w, req.ID, 1, fmt.Sprintf("no such container or image: %s", id))
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
		if configData, err := os.ReadFile(filepath.Join("/run/thrive/containers", entry.Name(), "config.json")); err == nil {
			var cfg map[string]any
			if err := json.Unmarshal(configData, &cfg); err == nil {
				if img, ok := cfg["Image"].(string); ok {
					state["image"] = img
				}
			}
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
		full := filepath.Join(mergedDir, srcPath)
		if st, err := os.Stat(full); err == nil && st.IsDir() {
			var buf bytes.Buffer
			if err := registry.TarDirectory(full, &buf); err != nil {
				sendError(w, req.ID, 1, fmt.Sprintf("cp from: tar: %v", err))
				return
			}
			writeResponse(w, &Response{
				ID:     req.ID,
				Result: map[string]any{"data": encb64.StdEncoding.EncodeToString(buf.Bytes()), "tar": true},
			})
			return
		}
		data, err := os.ReadFile(full)
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
		if isTar, _ := req.Opts["tar"].(bool); isTar {
			if err := os.MkdirAll(target, 0755); err != nil {
				sendError(w, req.ID, 1, fmt.Sprintf("cp to: mkdir: %v", err))
				return
			}
			if err := registry.ExtractArchive(bytes.NewReader(fileData), target); err != nil {
				sendError(w, req.ID, 1, fmt.Sprintf("cp to: extract: %v", err))
				return
			}
			writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ok": true}})
			return
		}
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

func handleCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "create requires image argument")
		return
	}
	imageRef := req.Args[0]
	cmdArgs := req.Args[1:]
	imgPath := "/var/lib/thrive/images/" + image.SafeRef(imageRef) + "/manifest.json"
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		if _, pullErr := image.Pull(ctx, imageRef, image.PullOptions{}); pullErr != nil {
			sendError(w, req.ID, 1, fmt.Sprintf("image %s not found: %v", imageRef, pullErr))
			return
		}
	}
	name, _ := req.Opts["name"].(string)
	containerID := name
	if containerID == "" {
		containerID = generateID()
	}
	var envVars []string
	if envRaw, ok := req.Opts["env"].([]any); ok {
		for _, e := range envRaw {
			if s, ok := e.(string); ok {
				envVars = append(envVars, s)
			}
		}
	}
	cfg := runtime.ContainerConfig{ID: containerID, Image: imageRef, Command: cmdArgs, Env: envVars}
	if _, err := runtime.Create(ctx, cfg); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"container_id": containerID}})
}

func handlePause(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "pause requires container ID")
		return
	}
	if err := runtime.Pause(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleUnpause(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "unpause requires container ID")
		return
	}
	if err := runtime.Unpause(ctx, req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleWait(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "wait requires container ID")
		return
	}
	code, err := runtime.Wait(ctx, req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"exit_code": code}})
}

func handleRename(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "rename requires old and new IDs")
		return
	}
	if err := runtime.Rename(ctx, req.Args[0], req.Args[1]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleStats(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "stats requires container ID")
		return
	}
	s, err := runtime.Stats(ctx, req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"id": s.ID, "status": s.Status, "pid": s.PID,
		"memory_current": s.MemoryCurrent, "memory_limit": s.MemoryLimit,
		"cpu_usage_usec": s.CPUUsageUsec, "pids_current": s.PIDsCurrent,
		"frozen": s.Frozen,
	}})
}

func handleUpdate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "update requires container ID")
		return
	}
	var opts runtime.UpdateOptions
	if v, ok := req.Opts["memory"].(string); ok && v != "" {
		opts.MemoryLimit = parseMemoryOpt(v)
	}
	if v, ok := req.Opts["cpu_quota"].(float64); ok {
		opts.CPUQuota = int64(v)
	}
	if v, ok := req.Opts["cpu_shares"].(float64); ok {
		opts.CPUShares = int64(v)
	}
	if v, ok := req.Opts["pids_limit"].(float64); ok {
		opts.PIDsLimit = int64(v)
	}
	if err := runtime.Update(ctx, req.Args[0], opts); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleTop(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "top requires container ID")
		return
	}
	procs, err := runtime.Top(ctx, req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, p := range procs {
		out = append(out, map[string]any{"pid": p.PID, "ppid": p.PPID, "cmd": p.Cmd})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"processes": out}})
}

func handlePort(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "port requires container ID")
		return
	}
	ports, err := runtime.ContainerPorts(ctx, req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, pm := range ports {
		out = append(out, map[string]any{
			"host_port": pm.HostPort, "container_port": pm.ContainerPort, "protocol": pm.Protocol,
		})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ports": out}})
}

func handleDiff(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "diff requires container ID")
		return
	}
	changes, err := runtime.Diff(ctx, req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, c := range changes {
		out = append(out, map[string]any{"kind": c.Kind, "path": c.Path})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"changes": out}})
}

func handleExport(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "export requires container ID")
		return
	}
	var buf bytes.Buffer
	if err := runtime.Export(ctx, req.Args[0], &buf); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"data": encb64.StdEncoding.EncodeToString(buf.Bytes()),
	}})
}

func handleCommit(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "commit requires container ID and image ref")
		return
	}
	opts := runtime.CommitOptions{Pause: true}
	if author, _ := req.Opts["author"].(string); author != "" {
		opts.Author = author
	}
	if message, _ := req.Opts["message"].(string); message != "" {
		opts.Message = message
	}
	if pause, ok := req.Opts["pause"].(bool); ok {
		opts.Pause = pause
	}
	if err := runtime.CommitWithOptions(ctx, req.Args[0], req.Args[1], opts); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ref": req.Args[1]}})
}

func handleSave(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "save requires at least one image")
		return
	}
	var buf bytes.Buffer
	if err := registry.SaveTo(ctx, image.StoreDir(), req.Args, &buf); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"data": encb64.StdEncoding.EncodeToString(buf.Bytes()),
	}})
}

func handleLoad(ctx context.Context, req *Request, w io.Writer) {
	encoded, _ := req.Opts["data"].(string)
	raw, err := base64Decode(encoded)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("load: decode: %v", err))
		return
	}
	loaded, err := registry.LoadFrom(ctx, image.StoreDir(), bytes.NewReader(raw))
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var refs []any
	for _, r := range loaded {
		refs = append(refs, r)
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"images": refs}})
}

func handleImport(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "import requires an image reference")
		return
	}
	encoded, _ := req.Opts["data"].(string)
	raw, err := base64Decode(encoded)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("import: decode: %v", err))
		return
	}
	if err := registry.ImportTo(ctx, image.StoreDir(), bytes.NewReader(raw), req.Args[0]); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ref": req.Args[0]}})
}

func handleHistory(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "history requires an image reference")
		return
	}
	layers, err := registry.HistoryTo(ctx, image.StoreDir(), req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, l := range layers {
		out = append(out, map[string]any{"digest": l.Digest, "size": l.Size})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"layers": out}})
}

func handleNetworkCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "network create requires a name")
		return
	}
	driver, _ := req.Opts["driver"].(string)
	subnet, _ := req.Opts["subnet"].(string)
	nw, err := network.CreateNetwork(req.Args[0], driver, subnet)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": nw.Name, "subnet": nw.Subnet}})
}

func handleNetworkLs(ctx context.Context, req *Request, w io.Writer) {
	nets, err := network.ListNetworks()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, nw := range nets {
		out = append(out, map[string]any{
			"name": nw.Name, "driver": nw.Driver,
			"subnet": nw.Subnet, "containers": len(nw.Containers),
		})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"networks": out}})
}

func handleNetworkInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "network inspect requires a name")
		return
	}
	nw, err := network.InspectNetwork(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"name": nw.Name, "driver": nw.Driver, "subnet": nw.Subnet,
		"gateway": nw.Gateway, "bridge": nw.Bridge, "containers": nw.Containers,
	}})
}

func handleNetworkRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "network rm requires a name")
		return
	}
	force, _ := req.Opts["force"].(bool)
	for _, name := range req.Args {
		if err := network.RemoveNetwork(name, force); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleNetworkConnect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "network connect requires network and container")
		return
	}
	networkName, containerID := req.Args[0], req.Args[1]
	if _, err := network.InspectNetwork(networkName); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	state, err := runtime.State(ctx, containerID)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}
	if state.Status != "running" {
		if err := runtime.AddPendingNetwork(containerID, networkName); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
		if err := network.ConnectNetwork(containerID, networkName); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
		writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"pending": true}})
		return
	}
	nw, _ := network.InspectNetwork(networkName)
	iface := fmt.Sprintf("eth%d", len(network.Attachments(containerID)))
	if err := network.EnsureBridgeWith(nw.Bridge, nw.Gateway+"/16"); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	veth, err := network.SetupVethOn(nw, containerID, state.PID, iface)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	if err := network.ConnectNetwork(containerID, networkName); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	network.RecordAttachment(containerID, network.Attachment{
		Network: nw.Name, HostVeth: veth.Host,
		Interface: iface, ContainerIP: veth.ContainerIP,
	})
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"ip": veth.ContainerIP}})
}

func handleNetworkDisconnect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 2 {
		sendError(w, req.ID, 1, "network disconnect requires network and container")
		return
	}
	networkName, containerID := req.Args[0], req.Args[1]
	force, _ := req.Opts["force"].(bool)
	state, err := runtime.State(ctx, containerID)
	if err != nil {
		sendError(w, req.ID, 1, fmt.Sprintf("container not found: %v", err))
		return
	}
	if state.Status == "running" && !force {
		for _, a := range network.Attachments(containerID) {
			if a.Network == networkName && a.Interface == "eth0" {
				sendError(w, req.ID, 1, "cannot disconnect primary network without --force")
				return
			}
		}
	}
	nw, err := network.InspectNetwork(networkName)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	for _, a := range network.Attachments(containerID) {
		if a.Network == networkName {
			network.TeardownVethOn(nw, containerID, a.HostVeth)
		}
	}
	var remaining []network.Attachment
	for _, a := range network.Attachments(containerID) {
		if a.Network != networkName {
			remaining = append(remaining, a)
		}
	}
	network.ClearAttachments(containerID)
	for _, a := range remaining {
		network.RecordAttachment(containerID, a)
	}
	if err := network.DisconnectNetwork(containerID, networkName); err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleNetworkPrune(ctx context.Context, req *Request, w io.Writer) {
	removed, err := network.PruneNetworks()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"removed": len(removed)}})
}

func daemonVolumeInUse(path string) bool {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/run/thrive/containers", e.Name(), "config.json"))
		if err != nil {
			continue
		}
		var cfg runtime.ContainerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		for _, m := range cfg.Mounts {
			if m.Source == path {
				return true
			}
		}
	}
	return false
}

func handleVolumeCreate(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "volume create requires a name")
		return
	}
	v, err := volume.Create(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": v.Name, "path": v.Path}})
}

func handleVolumeLs(ctx context.Context, req *Request, w io.Writer) {
	vols, err := volume.List()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, v := range vols {
		out = append(out, map[string]any{"name": v.Name, "path": v.Path})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"volumes": out}})
}

func handleVolumeInspect(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "volume inspect requires a name")
		return
	}
	v, err := volume.Inspect(req.Args[0])
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"name": v.Name, "path": v.Path}})
}

func handleVolumeRm(ctx context.Context, req *Request, w io.Writer) {
	if len(req.Args) < 1 {
		sendError(w, req.ID, 1, "volume rm requires a name")
		return
	}
	force, _ := req.Opts["force"].(bool)
	for _, name := range req.Args {
		if err := volume.Remove(name, force, daemonVolumeInUse); err != nil {
			sendError(w, req.ID, 1, err.Error())
			return
		}
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{}})
}

func handleVolumePrune(ctx context.Context, req *Request, w io.Writer) {
	removed, err := volume.Prune(daemonVolumeInUse)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"removed": len(removed)}})
}

func handleSystemDf(ctx context.Context, req *Request, w io.Writer) {
	u, err := thrsystem.DiskUsage()
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"images":     map[string]any{"total": u.Images.Total, "active": u.Images.Active, "size": u.Images.Size, "reclaimable": u.Images.Reclaimable},
		"containers": map[string]any{"total": u.Containers.Total, "active": u.Containers.Active, "size": u.Containers.Size, "reclaimable": u.Containers.Reclaimable},
		"volumes":    map[string]any{"total": u.Volumes.Total, "active": u.Volumes.Active, "size": u.Volumes.Size, "reclaimable": u.Volumes.Reclaimable},
		"networks":   map[string]any{"total": u.Networks.Total, "active": u.Networks.Active},
		"buildCache": map[string]any{"size": u.BuildCache.Size, "reclaimable": u.BuildCache.Reclaimable},
	}})
}

func handleSystemEvents(ctx context.Context, req *Request, w io.Writer) {
	var filter events.Filter
	now := time.Now().UTC()
	if s, _ := req.Opts["since"].(string); s != "" {
		if t, err := parseEventTimeOpt(s, now); err == nil {
			filter.Since = t
		}
	}
	if s, _ := req.Opts["until"].(string); s != "" {
		if t, err := parseEventTimeOpt(s, now); err == nil {
			filter.Until = t
		}
	}
	if s, _ := req.Opts["filter"].(string); s != "" {
		filter.Type = strings.TrimPrefix(strings.TrimPrefix(s, "type="), "event=")
	}
	list, err := events.Query(filter)
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	var out []map[string]any
	for _, e := range list {
		out = append(out, map[string]any{
			"time": e.Time.Format(time.RFC3339), "type": e.Type,
			"action": e.Action, "actor": e.Actor, "attributes": e.Attributes,
		})
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{"events": out}})
}

func handleSystemPrune(ctx context.Context, req *Request, w io.Writer) {
	all, _ := req.Opts["all"].(bool)
	volumes, _ := req.Opts["volumes"].(bool)
	rep, err := thrsystem.Prune(thrsystem.PruneOptions{AllImages: all, Volumes: volumes})
	if err != nil {
		sendError(w, req.ID, 1, err.Error())
		return
	}
	writeResponse(w, &Response{ID: req.ID, Result: map[string]any{
		"containersDeleted": rep.ContainersDeleted, "imagesDeleted": rep.ImagesDeleted,
		"volumesDeleted": rep.VolumesDeleted, "networksDeleted": rep.NetworksDeleted,
		"spaceReclaimed": rep.SpaceReclaimed,
	}})
}

func parseEventTimeOpt(s string, now time.Time) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}

// resourceLimitsFromOpts parses memory/cpus/shares/pids daemon opts.
func resourceLimitsFromOpts(opts map[string]any) runtime.ResourceLimits {
	var r runtime.ResourceLimits
	if v, ok := opts["memory"].(string); ok && v != "" {
		r.MemoryLimit = parseMemoryOpt(v)
	}
	if v, ok := opts["cpus"].(float64); ok && v > 0 {
		r.CPUQuota = int64(v * 100000)
	}
	if v, ok := opts["cpu_shares"].(float64); ok && v > 0 {
		r.CPUShares = int64(v)
	}
	if v, ok := opts["pids_limit"].(float64); ok && v > 0 {
		r.PIDsLimit = int64(v)
	}
	return r
}

// tailCount extracts an optional --tail line count from the request.
func tailCount(req *Request) (int, bool) {
	switch v := req.Opts["tail"].(type) {
	case float64:
		if v > 0 {
			return int(v), true
		}
	case int:
		if v > 0 {
			return v, true
		}
	}
	return 0, false
}

// lastLines returns the last n lines of data.
func lastLines(data []byte, n int) []byte {
	s := strings.TrimRight(string(data), "\n")
	if s == "" {
		return data
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func parseMemoryOpt(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	mult := int64(1)
	for _, suf := range []struct {
		suf string
		m   int64
	}{{"gb", 1 << 30}, {"g", 1 << 30}, {"mb", 1 << 20}, {"m", 1 << 20}, {"kb", 1 << 10}, {"k", 1 << 10}, {"b", 1}} {
		if strings.HasSuffix(s, suf.suf) {
			mult = suf.m
			s = strings.TrimSuffix(s, suf.suf)
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}
