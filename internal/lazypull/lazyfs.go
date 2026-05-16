//go:build linux
// +build linux

package lazypull

import (
	"context"
	"syscall"
	"sync"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type LazyFS struct {
	imageRef    string
	chunkStore  *image.ChunkStore
	fetcher     *ChunkFetcher
	manifest    *imageManifest
	root        *rootNode
	mu          sync.RWMutex
}

type imageManifest struct {
	layers []layerInfo
}

type layerInfo struct {
	digest string
	size   int64
}

func newLazyFS(imageRef string, layers []layerInfo) *LazyFS {
	cs := image.NewChunkStore("/var/lib/thrive/chunks")
	lf := &LazyFS{
		imageRef:   imageRef,
		chunkStore: cs,
		manifest:  &imageManifest{layers: layers},
	}
	lf.fetcher = newChunkFetcher(imageRef, cs)
	return lf
}

func (lf *LazyFS) Serve(mountPath string) error {
	log := telemetry.Logger()
	log.Info("lazyfs.Serve: starting", telemetry.FieldString("mountPath", mountPath), telemetry.FieldString("imageRef", lf.imageRef))

	root := &rootNode{lazy: lf}
	lf.root = root

	server, err := fs.Mount(mountPath, root, &fs.Options{
		MountOptions: fuse.MountOptions{
			FsName: "thrive-lazy",
		},
	})
	if err != nil {
		log.Error("lazyfs.Serve: fs.Mount failed", telemetry.FieldError(err))
		return err
	}

	log.Info("lazyfs.Serve: FUSE server running", telemetry.FieldString("mountPath", mountPath))
	server.Wait()
	return nil
}

type rootNode struct {
	fs.Inode
	lazy *LazyFS
}

var _ = fs.NodeOnAdder((*rootNode)(nil))

func (r *rootNode) OnAdd(ctx context.Context) {
	log := telemetry.Logger()
	log.Debug("rootNode.OnAdd: populating filesystem", telemetry.FieldString("imageRef", r.lazy.imageRef))

	for i, layer := range r.lazy.manifest.layers {
		layerNode := &layerDirNode{
			lazy:     r.lazy,
			digest:   layer.digest,
			size:     layer.size,
			layerIdx: i,
		}
		name := formatLayerName(i)
		r.AddChild(name, r.NewPersistentInode(ctx, layerNode, fs.StableAttr{Mode: fuse.S_IFDIR}), true)
	}
}

type layerDirNode struct {
	fs.Inode
	lazy     *LazyFS
	digest   string
	size     int64
	layerIdx int
}

var _ = fs.NodeOnAdder((*layerDirNode)(nil))

func (d *layerDirNode) OnAdd(ctx context.Context) {
	fileNode := &layerFileNode{
		lazy:   d.lazy,
		digest: d.digest,
		size:   d.size,
	}
	d.AddChild("layer.tar", d.NewPersistentInode(ctx, fileNode, fs.StableAttr{Mode: fuse.S_IFREG}), true)
}

type layerFileNode struct {
	fs.Inode
	lazy   *LazyFS
	digest string
	size   int64
}

var _ fs.NodeGetattrer = (*layerFileNode)(nil)
var _ fs.FileReader = (*layerFileNode)(nil)

func (f *layerFileNode) Getattr(ctx context.Context, fh fs.FileHandle, a *fuse.AttrOut) syscall.Errno {
	a.Size = uint64(f.size)
	a.Mode = fuse.S_IFREG | 0444
	return 0
}

func (f *layerFileNode) Read(ctx context.Context, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	log := telemetry.Logger()
	log.Debug("layerFileNode.Read: reading chunk", telemetry.FieldString("digest", f.digest[:12]))

	data, err := f.lazy.chunkStore.Get(ctx, f.digest)
	if err == nil {
		log.Debug("layerFileNode.Read: chunk hit", telemetry.FieldString("digest", f.digest[:12]))
		off := int(offset)
		if off >= len(data) {
			return fuse.ReadResultData([]byte{}), 0
		}
		end := off + len(dest)
		if end > len(data) {
			end = len(data)
		}
		return fuse.ReadResultData(data[off:end]), 0
	}

	log.Info("layerFileNode.Read: chunk miss, fetching", telemetry.FieldString("digest", f.digest[:12]))
	f.lazy.fetcher.Enqueue(f.digest)

	data, err = f.lazy.chunkStore.Get(ctx, f.digest)
	if err != nil {
		log.Error("layerFileNode.Read: chunk fetch failed", telemetry.FieldString("digest", f.digest[:12]), telemetry.FieldError(err))
		return nil, syscall.ENOENT
	}

	off := int(offset)
	if off >= len(data) {
		return fuse.ReadResultData([]byte{}), 0
	}
	end := off + len(dest)
	if end > len(data) {
		end = len(data)
	}
	return fuse.ReadResultData(data[off:end]), 0
}

func formatLayerName(idx int) string {
	return "sha256-" + formatDigestPrefix(idx)
}

func formatDigestPrefix(idx int) string {
	return "0000000000000000000000000000000000000000000000000000000000000000"[idx*8 : (idx+1)*8]
}