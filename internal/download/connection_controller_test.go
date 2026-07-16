package download

import "testing"

func TestConnectionControllerAdaptsAfterServerOverload(t *testing.T) {
	controller := newConnectionController(8)
	controller.overload()
	controller.overload()
	controller.mu.Lock()
	limit := controller.limit
	controller.mu.Unlock()
	if limit != 2 {
		t.Fatalf("adapted limit = %d, want 2", limit)
	}
	controller.success()
	controller.success()
	controller.mu.Lock()
	limit = controller.limit
	controller.mu.Unlock()
	if limit != 3 {
		t.Fatalf("recovered limit = %d, want 3", limit)
	}
}
