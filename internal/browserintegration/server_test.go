package browserintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBridgeRequiresTokenAndAcceptsOnlyAfterSubmit(t *testing.T) {
	calls := 0
	server, err := StartServer(t.TempDir(), func(context.Context, Request) error { calls++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	connection, err := ReadConnection(server.connectionPath)
	if err != nil {
		t.Fatal(err)
	}
	message := Request{Version: 1, RequestID: "x", Type: "add", URL: "https://example.test/a.zip"}
	payload, _ := json.Marshal(message)
	url := "http://127.0.0.1:" + httpPort(connection.Port) + "/v1/native"
	response, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+connection.Token)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted || calls != 1 {
		t.Fatalf("status=%d calls=%d", response.StatusCode, calls)
	}
	if _, err := ReadConnection(filepath.Join(filepath.Dir(server.connectionPath), "missing")); err == nil {
		t.Fatal("expected missing connection error")
	}
}
func httpPort(value int) string { return strconv.Itoa(value) }
