// Package provider holds the provider registry, the cost table and the
// transport-error classification shared by every model backend.
//
// Concrete backends live in sub-packages (anthropic, openaicompat, fake) and
// implement model.Provider. This package never performs I/O.
package provider

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/joeyvictorino/assay/internal/model"
)

// ErrUnknownProvider is returned by Registry.Get for an unregistered name.
var ErrUnknownProvider = errors.New("provider: unknown provider")

// Registry maps provider names to implementations. It is safe for
// concurrent use.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]model.Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]model.Provider{}}
}

// Register adds p under name. Re-registering a name replaces it.
func (r *Registry) Register(name string, p model.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Get returns the provider registered under name.
func (r *Registry) Get(name string) (model.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	return p, nil
}

// Names returns the registered provider names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
