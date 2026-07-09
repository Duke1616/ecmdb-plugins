package web

import (
	"context"
	"fmt"
	"sync"

	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/Duke1616/vuefinder-go/pkg/provider"
	sftpprovider "github.com/Duke1616/vuefinder-go/pkg/provider/sftp"
	vuefinderweb "github.com/Duke1616/vuefinder-go/pkg/web"
)

const sftpStorageName = "sftp"

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

func (r *finderRuntime) attach(sessionID int64, sess term.Session) error {
	fileCapable, ok := sess.(term.FileCapable)
	if !ok {
		return fmt.Errorf("session does not implement FileCapable")
	}

	client, err := fileCapable.NewSFTP()
	if err != nil {
		return err
	}

	r.SetFinder(sessionID, singleStorageSFTPProvider{inner: sftpprovider.NewSftpFinder(client)})
	r.markReady(sessionID)
	return nil
}

func (r *finderRuntime) markReady(sessionID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready[sessionID] = struct{}{}
}

func (r *finderRuntime) isReady(sessionID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.ready[sessionID]
	return ok
}

func (r *finderRuntime) clear(sessionID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ready, sessionID)
}

type singleStorageSFTPProvider struct {
	inner provider.CapabilityProvider
}

func (p singleStorageSFTPProvider) Caps() *provider.Capabilities {
	caps := p.inner.Caps()
	if caps == nil {
		return nil
	}

	if caps.Lister != nil {
		caps.Lister = singleStorageLister{inner: caps.Lister}
	}
	if caps.Searcher != nil {
		caps.Searcher = singleStorageSearcher{inner: caps.Searcher}
	}
	return caps
}

type singleStorageLister struct {
	inner provider.Lister
}

func (l singleStorageLister) Index(ctx context.Context, path string) (finder.Storages, error) {
	res, err := l.inner.Index(ctx, path)
	if err != nil {
		return finder.Storages{}, err
	}
	res.Storages = []string{sftpStorageName}
	return res, nil
}

type singleStorageSearcher struct {
	inner provider.Searcher
}

func (s singleStorageSearcher) Search(ctx context.Context, adapter, path, filter string) (finder.Storages, error) {
	res, err := s.inner.Search(ctx, adapter, path, filter)
	if err != nil {
		return finder.Storages{}, err
	}
	res.Storages = []string{sftpStorageName}
	return res, nil
}
