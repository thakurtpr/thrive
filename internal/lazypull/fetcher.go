//go:build linux
// +build linux

package lazypull

import (
	"context"
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

	log.Debug("ChunkFetcher.fetchChunk: chunk not in store, would fetch from registry", telemetry.FieldString("digest", digest[:12]))
	return nil
}

func (cf *ChunkFetcher) Shutdown() {
	log := telemetry.Logger()
	log.Info("ChunkFetcher.Shutdown: initiating")
	cf.cancel()
	cf.wg.Wait()
	log.Info("ChunkFetcher.Shutdown: complete")
}