package eventing_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
)

func TestBusDelivers(t *testing.T) {
	bus := eventing.NewInMemoryBus()
	ctx := context.Background()

	var mu sync.Mutex
	var received []eventing.Event

	bus.Subscribe(eventing.TopicUserCreated, func(ctx context.Context, e eventing.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	evt := eventing.Event{Topic: eventing.TopicUserCreated, Payload: eventing.UserCreated{UserID: "u-1"}}
	if err := bus.Publish(ctx, evt); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Allow async delivery.
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	uc, ok := received[0].Payload.(eventing.UserCreated)
	if !ok || uc.UserID != "u-1" {
		t.Fatalf("unexpected payload: %+v", received[0].Payload)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	bus := eventing.NewInMemoryBus()
	ctx := context.Background()

	var mu sync.Mutex
	count := 0

	sub := bus.Subscribe(eventing.TopicUserCreated, func(ctx context.Context, e eventing.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	evt := eventing.Event{Topic: eventing.TopicUserCreated, Payload: eventing.UserCreated{UserID: "u-2"}}
	_ = bus.Publish(ctx, evt)
	time.Sleep(20 * time.Millisecond)

	bus.Unsubscribe(sub)

	_ = bus.Publish(ctx, evt)
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected count=1 after unsubscribe, got %d", count)
	}
}
