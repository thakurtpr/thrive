//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shortTempHome creates a temp dir under /tmp to stay under the 104-byte
// Unix socket path limit macOS imposes on AF_UNIX addresses.
func shortTempHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "thrive_test_")
	if err != nil {
		t.Fatalf("shortTempHome: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestPrepareVSOCKListener_CreatesSocketAndStoresListener(t *testing.T) {
	home := shortTempHome(t)
	t.Setenv("HOME", home)
	vmDir := filepath.Join(home, ".thrive", "vm")
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		t.Fatal(err)
	}
	vsockListener = nil
	t.Cleanup(CloseVSOCKListener)

	if err := PrepareVSOCKListener(); err != nil {
		t.Fatalf("PrepareVSOCKListener: %v", err)
	}
	if vsockListener == nil {
		t.Error("vsockListener should not be nil after PrepareVSOCKListener")
	}
	sockPath := filepath.Join(vmDir, "vsock.sock")
	if _, err := os.Stat(sockPath); err != nil {
		t.Errorf("socket file should exist at %s: %v", sockPath, err)
	}
}

func TestPrepareVSOCKListener_RemovesStaleSocket(t *testing.T) {
	home := shortTempHome(t)
	t.Setenv("HOME", home)
	vmDir := filepath.Join(home, ".thrive", "vm")
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		t.Fatal(err)
	}
	sockPath := filepath.Join(vmDir, "vsock.sock")
	if err := os.WriteFile(sockPath, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	vsockListener = nil
	t.Cleanup(CloseVSOCKListener)

	if err := PrepareVSOCKListener(); err != nil {
		t.Fatalf("PrepareVSOCKListener with stale file: %v", err)
	}
	if vsockListener == nil {
		t.Error("vsockListener should not be nil")
	}
}

func TestCloseVSOCKListener_NilsGlobal(t *testing.T) {
	home := shortTempHome(t)
	t.Setenv("HOME", home)
	vmDir := filepath.Join(home, ".thrive", "vm")
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		t.Fatal(err)
	}
	vsockListener = nil
	if err := PrepareVSOCKListener(); err != nil {
		t.Fatalf("PrepareVSOCKListener: %v", err)
	}

	CloseVSOCKListener()

	if vsockListener != nil {
		t.Error("vsockListener should be nil after CloseVSOCKListener")
	}
}

func TestNewVSOCKBridge_AutoPreparesWhenListenerNil(t *testing.T) {
	// Auto-prepare creates vsock.sock. With no VM, Accept times out.
	// The error should NOT be "PrepareVSOCKListener not called".
	vsockListener = nil
	_, err := newVSOCKBridge()
	if err == nil {
		t.Fatal("expected timeout error (no VM running), got nil")
	}
	if strings.Contains(err.Error(), "PrepareVSOCKListener not called") {
		t.Errorf("auto-prepare should work without manual call, got: %v", err)
	}
	vsockListener = nil
}

func TestVSOCKBridge_Exec_SendsJSONAndDecodesResult(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bridge := &vsockBridge{conn: client, dec: json.NewDecoder(client)}

	go func() {
		var req map[string]any
		json.NewDecoder(server).Decode(&req)
		resp := map[string]any{"result": "pong"}
		data, _ := json.Marshal(resp)
		server.Write(append(data, '\n'))
	}()

	result, err := bridge.Exec(context.Background(), "ping", nil, nil)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if string(result) != `"pong"` {
		t.Errorf("expected result %q, got %s", "pong", result)
	}
}

func TestVSOCKBridge_Exec_ReturnsDaemonError(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bridge := &vsockBridge{conn: client, dec: json.NewDecoder(client)}

	go func() {
		var req map[string]any
		json.NewDecoder(server).Decode(&req)
		resp := map[string]any{"error": map[string]any{"message": "command not found"}}
		data, _ := json.Marshal(resp)
		server.Write(append(data, '\n'))
	}()

	_, err := bridge.Exec(context.Background(), "badcmd", nil, nil)
	if err == nil {
		t.Fatal("expected daemon error, got nil")
	}
	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("expected 'command not found' in error, got: %v", err)
	}
}

func TestVSOCKBridge_ExecStream_CollectsLinesUntilEOF(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bridge := &vsockBridge{conn: client, dec: json.NewDecoder(client)}

	go func() {
		var req map[string]any
		json.NewDecoder(server).Decode(&req)
		for _, line := range []string{"line1", "line2", "line3"} {
			resp := map[string]any{"stream": line}
			data, _ := json.Marshal(resp)
			server.Write(append(data, '\n'))
		}
		eof := map[string]any{"eof": true}
		data, _ := json.Marshal(eof)
		server.Write(append(data, '\n'))
	}()

	var buf strings.Builder
	if err := bridge.ExecStream(context.Background(), "logs", nil, nil, &buf); err != nil {
		t.Fatalf("ExecStream failed: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"line1", "line2", "line3"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %q", want, got)
		}
	}
}
