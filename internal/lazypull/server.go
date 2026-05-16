//go:build linux
// +build linux

package lazypull

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type Server struct {
	mountPath string
	lazyFS    *LazyFS
	fuseCmd   *exec.Cmd
	wg        sync.WaitGroup
	mu        sync.RWMutex
	started   bool
}

func newServer(mountPath string, imageRef string, layers []layerInfo) *Server {
	return &Server{
		mountPath: mountPath,
		lazyFS:    newLazyFS(imageRef, layers),
	}
}

func (s *Server) Serve() error {
	log := telemetry.Logger()
	log.Info("Server.Serve: starting FUSE mount", telemetry.FieldString("mountPath", s.mountPath))

	if err := os.MkdirAll(s.mountPath, 0755); err != nil {
		log.Error("Server.Serve: mkdir failed", telemetry.FieldString("path", s.mountPath), telemetry.FieldError(err))
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.lazyFS.Serve(s.mountPath); err != nil {
			log.Error("Server.Serve: lazyFS.Serve error", telemetry.FieldError(err))
		}
	}()

	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	return nil
}

func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) Unmount() error {
	log := telemetry.Logger()
	log.Info("Server.Unmount: unmounting", telemetry.FieldString("mountPath", s.mountPath))

	s.lazyFS.fetcher.Shutdown()

	fusermount := exec.Command("fusermount3", "-u", s.mountPath)
	if err := fusermount.Run(); err != nil {
		log.Error("Server.Unmount: fusermount3 failed", telemetry.FieldString("path", s.mountPath), telemetry.FieldError(err))
		return err
	}

	log.Info("Server.Unmount: success", telemetry.FieldString("mountPath", s.mountPath))
	return nil
}

func MountLazy(imageRef string, mountPath string, layers []layerInfo) (*Server, error) {
	log := telemetry.Logger()
	log.Info("MountLazy: creating lazy mount server", telemetry.FieldString("imageRef", imageRef), telemetry.FieldString("mountPath", mountPath))

	dir := filepath.Dir(mountPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error("MountLazy: parent dir failed", telemetry.FieldString("dir", dir), telemetry.FieldError(err))
		return nil, err
	}

	srv := newServer(mountPath, imageRef, layers)
	if err := srv.Serve(); err != nil {
		log.Error("MountLazy: server start failed", telemetry.FieldError(err))
		return nil, err
	}

	log.Info("MountLazy: server started", telemetry.FieldString("mountPath", mountPath))
	return srv, nil
}