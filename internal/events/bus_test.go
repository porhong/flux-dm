package events

import "testing"

func TestBusPublishesAndUnsubscribes(t *testing.T) {
	bus := NewBus()
	count := 0
	unsubscribe := bus.Subscribe(AppReady, func(event Event) {
		if event.Message != "ready" {
			t.Fatalf("unexpected message %q", event.Message)
		}
		count++
	})

	bus.Publish(Event{Type: AppReady, Message: "ready"})
	unsubscribe()
	bus.Publish(Event{Type: AppReady, Message: "ready"})

	if count != 1 {
		t.Fatalf("expected one delivery, got %d", count)
	}
}
