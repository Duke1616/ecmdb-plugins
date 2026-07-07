package web

import (
	"fmt"
	"sync"

	"github.com/Duke1616/ecmdb/pkg/term"
	sftpprovider "github.com/Duke1616/vuefinder-go/pkg/provider/sftp"
	vuefinderweb "github.com/Duke1616/vuefinder-go/pkg/web"
)

type finderRuntime struct {
	*vuefinderweb.Handler

	mu    sync.RWMutex
	ready map[int64]struct{}
}

func newFinderRuntime() *finderRuntime {
	return &finderRuntime{
		Handler: vuefinderweb.NewHandler(),
		ready:   make(map[int64]struct{}),
	}
}

func (r *finderRuntime) attach(resourceID int64, sess term.Session) error {
	fileCapable, ok := sess.(term.FileCapable)
	if !ok {
		return fmt.Errorf("session does not implement FileCapable")
	}

	client, err := fileCapable.NewSFTP()
	if err != nil {
		return err
	}

	r.SetFinder(resourceID, sftpprovider.NewSftpFinder(client))
	r.markReady(resourceID)
	return nil
}

func (r *finderRuntime) markReady(resourceID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready[resourceID] = struct{}{}
}

func (r *finderRuntime) isReady(resourceID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.ready[resourceID]
	return ok
}

func (r *finderRuntime) clear(resourceID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ready, resourceID)
}
