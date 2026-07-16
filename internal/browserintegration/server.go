package browserintegration

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Connection struct {
	Port      int    `json:"port"`
	Token     string `json:"token"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"`
}
type SubmitFunc func(context.Context, Request) error
type Server struct {
	listener       net.Listener
	httpServer     *http.Server
	connectionPath string
	token          string
	done           chan error
}

func StartServer(dataDir string, submit SubmitFunc) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen browser bridge: %w", err)
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		listener.Close()
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	server := &Server{listener: listener, connectionPath: filepath.Join(dataDir, "browser-bridge.json"), token: token, done: make(chan error, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/native", server.handle(submit))
	server.httpServer = &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := writeConnection(server.connectionPath, Connection{Port: port, Token: token, PID: os.Getpid(), StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		listener.Close()
		return nil, err
	}
	go func() { server.done <- server.httpServer.Serve(listener) }()
	return server, nil
}
func (s *Server) Close(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	connection, readErr := ReadConnection(s.connectionPath)
	if readErr == nil && connection.Token == s.token {
		_ = os.Remove(s.connectionPath)
	}
	if err != nil {
		return err
	}
	select {
	case serveErr := <-s.done:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
	default:
	}
	return nil
}
func (s *Server) handle(submit SubmitFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, MaxMessageBytes)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var message Request
		if err := decoder.Decode(&message); err != nil {
			writeJSON(writer, http.StatusBadRequest, Response{Version: 1, Code: "invalid_message", Message: "The browser request was invalid."})
			return
		}
		if err := ValidateRequest(message); err != nil {
			writeJSON(writer, http.StatusBadRequest, Response{Version: 1, RequestID: message.RequestID, Code: "invalid_request", Message: "The browser request was rejected."})
			return
		}
		if message.Type == "ping" {
			writeJSON(writer, http.StatusOK, Response{Version: 1, RequestID: message.RequestID, Accepted: true})
			return
		}
		if err := submit(request.Context(), message); err != nil {
			writeJSON(writer, http.StatusServiceUnavailable, Response{Version: 1, RequestID: message.RequestID, Code: "unavailable", Message: "FluxDM could not accept the download."})
			return
		}
		writeJSON(writer, http.StatusAccepted, Response{Version: 1, RequestID: message.RequestID, Accepted: true})
	}
}
func writeJSON(writer http.ResponseWriter, status int, value Response) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
func writeConnection(path string, value Connection) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + "." + strconv.Itoa(os.Getpid()) + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
func ReadConnection(path string) (Connection, error) {
	file, err := os.Open(path)
	if err != nil {
		return Connection{}, err
	}
	defer file.Close()
	limited := io.LimitReader(file, 4097)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return Connection{}, err
	}
	if len(payload) > 4096 {
		return Connection{}, fmt.Errorf("connection file too large")
	}
	var value Connection
	if err := json.Unmarshal(payload, &value); err != nil {
		return Connection{}, err
	}
	if value.Port < 1 || value.Port > 65535 || len(value.Token) < 32 {
		return Connection{}, fmt.Errorf("invalid connection file")
	}
	return value, nil
}
