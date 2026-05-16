//go:build linux
// +build linux

package lazypull

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type ChunkFetcher struct {
	imageRef   string
	chunkStore *image.ChunkStore
	queue      chan string
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func newChunkFetcher(imageRef string, cs *image.ChunkStore) *ChunkFetcher {
	ctx, cancel := context.WithCancel(context.Background())
	cf := &ChunkFetcher{
		imageRef:   imageRef,
		chunkStore: cs,
		queue:      make(chan string, 128),
		ctx:        ctx,
		cancel:     cancel,
	}
	cf.wg.Add(1)
	go cf.run()
	return cf
}

func (cf *ChunkFetcher) Enqueue(digest string) {
	log := telemetry.Logger()
	select {
	case cf.queue <- digest:
		log.Debug("ChunkFetcher.Enqueue: enqueued", telemetry.FieldString("digest", digest[:12]))
	default:
		log.Warn("ChunkFetcher.Enqueue: queue full, dropping", telemetry.FieldString("digest", digest[:12]))
	}
}

func (cf *ChunkFetcher) run() {
	defer cf.wg.Done()
	log := telemetry.Logger()
	log.Info("ChunkFetcher.run: starting background fetcher", telemetry.FieldString("imageRef", cf.imageRef))

	for {
		select {
		case <-cf.ctx.Done():
			log.Info("ChunkFetcher.run: shutting down")
			return
		case digest := <-cf.queue:
			log.Info("ChunkFetcher.run: fetching chunk", telemetry.FieldString("digest", digest[:12]))
			if err := cf.fetchChunk(digest); err != nil {
				log.Error("ChunkFetcher.run: fetch failed", telemetry.FieldString("digest", digest[:12]), telemetry.FieldError(err))
			}
		}
	}
}

func (cf *ChunkFetcher) fetchChunk(digest string) error {
	log := telemetry.Logger()

	if cf.chunkStore.Has(context.Background(), digest) {
		log.Debug("ChunkFetcher.fetchChunk: already cached", telemetry.FieldString("digest", digest[:12]))
		return nil
	}

	// Derive registry and repository from imageRef.
	// imageRef examples: "alpine", "library/alpine", "registry.example.com/myapp:v1"
	ref := cf.imageRef
	// Strip any tag.
	if idx := strings.LastIndex(ref, ":"); idx > strings.LastIndex(ref, "/") {
		ref = ref[:idx]
	}

	registry := "registry-1.docker.io"
	repo := ref
	if parts := strings.SplitN(ref, "/", 2); len(parts) == 2 && strings.Contains(parts[0], ".") {
		registry = parts[0]
		repo = parts[1]
	} else if !strings.Contains(ref, "/") {
		repo = "library/" + ref
	}

	blobURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, repo, digest)
	log.Info("ChunkFetcher.fetchChunk: fetching", telemetry.FieldString("digest", digest[:12]), telemetry.FieldString("url", blobURL))

	req, err := http.NewRequestWithContext(cf.ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return fmt.Errorf("ChunkFetcher.fetchChunk: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ChunkFetcher.fetchChunk: GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("ChunkFetcher.fetchChunk: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ChunkFetcher.fetchChunk: read body: %w", err)
	}

	if err := cf.chunkStore.Put(context.Background(), digest, data); err != nil {
		return fmt.Errorf("ChunkFetcher.fetchChunk: store: %w", err)
	}

	log.Info("ChunkFetcher.fetchChunk: stored", telemetry.FieldString("digest", digest[:12]), telemetry.FieldInt("bytes", len(data)))
	return nil
}

func (cf *ChunkFetcher) Shutdown() {
	log := telemetry.Logger()
	log.Info("ChunkFetcher.Shutdown: initiating")
	cf.cancel()
	cf.wg.Wait()
	log.Info("ChunkFetcher.Shutdown: complete")
}