package registry

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	protobuf_go_lite "github.com/aperturerobotics/protobuf-go-lite"
)

var ErrNotFound = errors.New("message type not found")

type Option struct {
	Name   string
	Values []string
}

type Entry struct {
	FullName string
	TypeURL  string
	New      func() protobuf_go_lite.Message
	Options  []Option
}

func (e Entry) Option(name string) (string, bool) {
	for _, opt := range e.Options {
		if opt.Name == name {
			if len(opt.Values) == 0 {
				return "", true
			}
			return opt.Values[0], true
		}
	}
	return "", false
}

type Registry struct {
	mu      sync.RWMutex
	byName  map[string]Entry
	byURL   map[string]Entry
	entries []Entry
}

func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Entry),
		byURL:  make(map[string]Entry),
	}
}

func (r *Registry) Register(e Entry) {
	if e.FullName == "" {
		panic("registry: Entry.FullName is required")
	}
	if e.New == nil {
		panic("registry: Entry.New is required")
	}
	if e.TypeURL == "" {
		e.TypeURL = "type.googleapis.com/" + e.FullName
	}

	e = copyEntry(e)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byName[e.FullName]; ok {
		panic(fmt.Sprintf("registry: duplicate registration for %q", e.FullName))
	}
	r.byName[e.FullName] = e
	r.byURL[e.TypeURL] = e
	r.byURL[e.FullName] = e
	r.entries = append(r.entries, e)
	sort.Slice(r.entries, func(i, j int) bool {
		return r.entries[i].FullName < r.entries[j].FullName
	})
}

func (r *Registry) All() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Entry, len(r.entries))
	for i, e := range r.entries {
		out[i] = copyEntry(e)
	}
	return out
}

func (r *Registry) Range(fn func(Entry) bool) {
	for _, e := range r.All() {
		if !fn(e) {
			return
		}
	}
}

func (r *Registry) NewByName(name string) (protobuf_go_lite.Message, bool) {
	r.mu.RLock()
	e, ok := r.byName[name]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return e.New(), true
}

func (r *Registry) NewByTypeURL(url string) (protobuf_go_lite.Message, bool) {
	r.mu.RLock()
	e, ok := r.lookupByTypeURLLocked(url)
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return e.New(), true
}

func (r *Registry) FindMessageByURL(url string) (func() protobuf_go_lite.Message, error) {
	r.mu.RLock()
	e, ok := r.lookupByTypeURLLocked(url)
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	newFn := e.New
	return newFn, nil
}

func (r *Registry) lookupByTypeURLLocked(url string) (Entry, bool) {
	if e, ok := r.byURL[url]; ok {
		return e, true
	}
	name := url
	if i := strings.LastIndex(url, "/"); i >= 0 {
		name = url[i+1:]
	}
	e, ok := r.byName[name]
	return e, ok
}

func copyEntry(e Entry) Entry {
	out := Entry{
		FullName: e.FullName,
		TypeURL:  e.TypeURL,
		New:      e.New,
	}
	if len(e.Options) > 0 {
		out.Options = make([]Option, len(e.Options))
		for i, opt := range e.Options {
			out.Options[i] = Option{
				Name:   opt.Name,
				Values: append([]string(nil), opt.Values...),
			}
		}
	}
	return out
}

var defaultRegistry = NewRegistry()

func Register(e Entry) { defaultRegistry.Register(e) }

func All() []Entry { return defaultRegistry.All() }

func Range(fn func(Entry) bool) { defaultRegistry.Range(fn) }

func NewByName(name string) (protobuf_go_lite.Message, bool) {
	return defaultRegistry.NewByName(name)
}

func NewByTypeURL(url string) (protobuf_go_lite.Message, bool) {
	return defaultRegistry.NewByTypeURL(url)
}

func Default() *Registry { return defaultRegistry }
