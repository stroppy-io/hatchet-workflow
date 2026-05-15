package eventing

import (
	"context"
	"sync"
)

// Topic identifies a category of events.
type Topic string

// Event is a message published on the bus.
type Event struct {
	Topic   Topic
	Payload any
}

// Handler is the callback invoked when an event is delivered.
type Handler func(ctx context.Context, e Event)

// Subscription is an opaque handle returned by Subscribe; pass it to Unsubscribe.
type Subscription struct {
	topic Topic
	id    uint64
}

// Bus is the eventing contract.
type Bus interface {
	Publish(ctx context.Context, e Event) error
	Subscribe(topic Topic, h Handler) Subscription
	Unsubscribe(sub Subscription)
}

type subscriber struct {
	id      uint64
	handler Handler
}

type inMemoryBus struct {
	mu   sync.RWMutex
	subs map[Topic][]subscriber
	seq  uint64
}

// NewInMemoryBus returns a Bus backed by in-process goroutines.
func NewInMemoryBus() Bus {
	return &inMemoryBus{subs: make(map[Topic][]subscriber)}
}

func (b *inMemoryBus) Publish(ctx context.Context, e Event) error {
	b.mu.RLock()
	handlers := make([]Handler, 0, len(b.subs[e.Topic]))
	for _, s := range b.subs[e.Topic] {
		handlers = append(handlers, s.handler)
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		h := h
		go h(ctx, e)
	}
	return nil
}

func (b *inMemoryBus) Subscribe(topic Topic, h Handler) Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := b.seq
	b.subs[topic] = append(b.subs[topic], subscriber{id: id, handler: h})
	return Subscription{topic: topic, id: id}
}

func (b *inMemoryBus) Unsubscribe(sub Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.subs[sub.topic]
	for i, s := range list {
		if s.id == sub.id {
			b.subs[sub.topic] = append(list[:i], list[i+1:]...)
			return
		}
	}
}
