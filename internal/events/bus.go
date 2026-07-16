package events

import "sync"

type Type string

const (
	AppReady         Type = "app.ready"
	DownloadProgress Type = "download.progress"
	DownloadUpdated  Type = "download.updated"
)

type Event struct {
	Type    Type
	Message string
	Data    any
}

type Handler func(Event)

// Bus is a small synchronous in-process event bus. Handlers must return quickly.
type Bus struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[Type]map[uint64]Handler
}

func NewBus() *Bus {
	return &Bus{subscribers: make(map[Type]map[uint64]Handler)}
}

func (b *Bus) Subscribe(eventType Type, handler Handler) func() {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.subscribers[eventType] == nil {
		b.subscribers[eventType] = make(map[uint64]Handler)
	}
	b.subscribers[eventType][id] = handler
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.subscribers[eventType], id)
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	handlers := make([]Handler, 0, len(b.subscribers[event.Type]))
	for _, handler := range b.subscribers[event.Type] {
		handlers = append(handlers, handler)
	}
	b.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}
