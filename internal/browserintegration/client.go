package browserintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

var bridgeHTTPClient = &http.Client{Timeout: 5 * time.Second}

func Forward(ctx context.Context, dataDir string, message Request) (Response, error) {
	connection, err := ReadConnection(filepath.Join(dataDir, "browser-bridge.json"))
	if err != nil {
		return Response{}, err
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return Response{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/v1/native", connection.Port), bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	request.Header.Set("Authorization", "Bearer "+connection.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := bridgeHTTPClient.Do(request)
	if err != nil {
		return Response{}, err
	}
	defer response.Body.Close()
	var result Response
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Response{}, err
	}
	if response.StatusCode >= 300 {
		return result, fmt.Errorf("bridge returned %d", response.StatusCode)
	}
	return result, nil
}
