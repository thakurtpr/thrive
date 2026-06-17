//go:build linux
// +build linux

// Package lazypull tests exercise the internal logic that can be tested without
// a live FUSE kernel mount. The exported MountLazy function requires actual FUSE
// kernel support and is covered only via integration/E2E testing.
//
// Testable surface in this file:
//   - ChunkFetcher lifecycle (newChunkFetcher, Enqueue, Shutdown)
//   - ChunkFetcher cache-hit path (pre-populated chunk store skips HTTP)
//   - ChunkFetcher HTTP fetch via an httptest server (transport override)
//   - formatLayerName / formatDigestPrefix helpers
package lazypull

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// ---------------------------------------------------------------------------
// testTransport rewrites every outgoing request to a given test-server URL.
// This lets us exercise fetchChunk's HTTP path without touching the public
// internet, by swapping http.DefaultTransport for the duration of each test.
// ---------------------------------------------------------------------------

type testTransport struct {
	base *http.Transport
	host string // host:port of the httptest server
}

func newTestTransport(serverURL string) *testTransport {
	// Strip "http://" prefix to get host:port.
	host := serverURL
	if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	}
	return &testTransport{
		base: http.DefaultTransport.(*http.Transport).Clone(),
		host: host,
	}
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = t.host
	cloned.Host = t.host
	return t.base.RoundTrip(cloned)
}

// ---------------------------------------------------------------------------
// formatLayerName / formatDigestPrefix
// ---------------------------------------------------------------------------

func TestFormatLayerName_Index0(t *testing.T) {
	// The source string is 64 zeros; index 0 → chars [0:8] = "00000000".
	got := formatLayerName(0)
	want := "sha256-00000000"
	if got != want {
		t.Errorf("formatLayerName(0) = %q, want %q", got, want)
	}
}

func TestFormatLayerName_Index1(t *testing.T) {
	// Index 1 → chars [8:16] = "00000000" (still zeros from same source string).
	got := formatLayerName(1)
	want := "sha256-00000000"
	if got != want {
		t.Errorf("formatLayerName(1) = %q, want %q", got, want)
	}
}

func TestFormatDigestPrefix_ReturnsEightChars(t *testing.T) {
	for i := 0; i < 7; i++ {
		got := formatDigestPrefix(i)
		if len(got) != 8 {
			t.Errorf("formatDigestPrefix(%d) length = %d, want 8", i, len(got))
		}
	}
}

// ---------------------------------------------------------------------------
// ChunkFetcher lifecycle
// ---------------------------------------------------------------------------

func TestChunkFetcher_ShutdownCompletesWithoutHang(t *testing.T) {
	// Arrange
	cs := image.NewChunkStore(t.TempDir())
	cf := newChunkFetcher("alpine:latest", cs)

	// Act — Shutdown must complete quickly (no pending fetches).
	done := make(chan struct{})
	go func() {
		cf.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not complete within 3 seconds")
	}
}

func TestChunkFetcher_EnqueueAfterShutdown_DoesNotPanic(t *testing.T) {
	// Arrange
	cs := image.NewChunkStore(t.TempDir())
	cf := newChunkFetcher("alpine:latest", cs)
	cf.Shutdown()

	// A 64-char digest so the [:12] log slice does not panic.
	digest := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899aa"
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Enqueue after Shutdown panicked: %v", r)
		}
	}()

	// Act — queue is full or context cancelled; must drop silently.
	cf.Enqueue(digest)
}

// ---------------------------------------------------------------------------
// ChunkFetcher cache-hit path
// ---------------------------------------------------------------------------

func TestChunkFetcher_FetchChunk_CacheHit_SkipsHTTP(t *testing.T) {
	// Arrange — pre-populate the chunk store; fetchChunk must return nil
	// without making any HTTP request.
	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	ctx := context.Background()

	payload := []byte("already cached layer data")
	digest := image.ComputeDigest(payload)

	if err := cs.Put(ctx, digest, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Wire an HTTP server that fails the test if contacted — the cache hit
	// must prevent any HTTP call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("HTTP server was called for a cached chunk (digest %s)", digest[:12])
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	// Act
	err := cf.fetchChunk(digest)

	// Assert
	if err != nil {
		t.Errorf("fetchChunk on cached chunk returned error: %v", err)
	}

	// Original data must be intact.
	got, err := cs.Get(ctx, digest)
	if err != nil {
		t.Fatalf("Get after cache-hit fetchChunk: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("cached data corrupted: got %q, want %q", got, payload)
	}
}

// ---------------------------------------------------------------------------
// ChunkFetcher HTTP fetch via httptest server
// ---------------------------------------------------------------------------

func TestChunkFetcher_FetchChunk_HTTPSuccess(t *testing.T) {
	// Arrange
	blobData := []byte("layer blob content from registry")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(blobData)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	digest := image.ComputeDigest(blobData)

	// Act
	err := cf.fetchChunk(digest)

	// Assert
	if err != nil {
		t.Fatalf("fetchChunk with HTTP 200 returned error: %v", err)
	}

	ctx := context.Background()
	got, err := cs.Get(ctx, digest)
	if err != nil {
		t.Fatalf("Get after fetchChunk: %v", err)
	}
	if string(got) != string(blobData) {
		t.Errorf("fetched data = %q, want %q", got, blobData)
	}
}

func TestChunkFetcher_FetchChunk_HTTP206_PartialContent(t *testing.T) {
	// Arrange — 206 Partial Content is also a success status for blob fetch.
	blobData := []byte("partial content blob")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(blobData)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	digest := image.ComputeDigest(blobData)

	// Act
	err := cf.fetchChunk(digest)

	// Assert — 206 must not produce an error
	if err != nil {
		t.Fatalf("fetchChunk with HTTP 206 returned error: %v", err)
	}

	ctx := context.Background()
	got, err := cs.Get(ctx, digest)
	if err != nil {
		t.Fatalf("Get after 206 fetchChunk: %v", err)
	}
	if string(got) != string(blobData) {
		t.Errorf("206 fetched data = %q, want %q", got, blobData)
	}
}

func TestChunkFetcher_FetchChunk_HTTPError_404(t *testing.T) {
	// Arrange — server returns 404.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	// Digest not in cache so the HTTP path is exercised.
	digest := image.ComputeDigest([]byte("not-in-cache-404"))

	// Act
	err := cf.fetchChunk(digest)

	// Assert — 404 must be returned as an error
	if err == nil {
		t.Errorf("fetchChunk with 404 should return error, got nil")
	}
}

func TestChunkFetcher_FetchChunk_CacheHitOnSecondCall(t *testing.T) {
	// Arrange — first call fetches via HTTP; second call must hit the cache.
	blobData := []byte("layer data for second-call test")

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(blobData)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	digest := image.ComputeDigest(blobData)

	// Act — two consecutive direct calls
	if err := cf.fetchChunk(digest); err != nil {
		t.Fatalf("first fetchChunk: %v", err)
	}
	if err := cf.fetchChunk(digest); err != nil {
		t.Fatalf("second fetchChunk: %v", err)
	}

	// Assert — HTTP hit exactly once; second call is a cache hit
	if callCount != 1 {
		t.Errorf("HTTP server hit %d times, want 1 (second call must be cache hit)", callCount)
	}
}

// ---------------------------------------------------------------------------
// Enqueue integration — verify enqueued digests are eventually fetched
// ---------------------------------------------------------------------------

func TestChunkFetcher_Enqueue_EventuallyFetches(t *testing.T) {
	// Arrange
	blobData := []byte("enqueue integration blob")

	fetched := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(blobData)
		select {
		case fetched <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = newTestTransport(srv.URL)
	defer func() { http.DefaultTransport = origTransport }()

	tmpDir := t.TempDir()
	cs := image.NewChunkStore(tmpDir)
	cf := newChunkFetcher("alpine:latest", cs)
	defer cf.Shutdown()

	digest := image.ComputeDigest(blobData)

	// Act — enqueue; the background goroutine picks it up.
	cf.Enqueue(digest)

	// Assert — wait for the background goroutine to call the HTTP server.
	select {
	case <-fetched:
		// pass
	case <-time.After(5 * time.Second):
		t.Fatal("background fetcher did not fetch the chunk within 5 seconds")
	}
}
