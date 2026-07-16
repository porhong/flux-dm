package testserver

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	HTTP           *httptest.Server
	Payload        []byte
	mutableVersion atomic.Int32
	requestMu      sync.Mutex
	requestCounts  map[string]int
}

func New() *Server {
	payload := []byte(strings.Repeat("FluxDM deterministic fixture\n", 4096))
	mux := http.NewServeMux()
	server := &Server{Payload: payload, requestCounts: make(map[string]int)}
	server.mutableVersion.Store(1)
	mux.HandleFunc("/file", server.file)
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/file", http.StatusFound) })
	mux.HandleFunc("/unknown", server.unknown)
	mux.HandleFunc("/slow", server.slow)
	mux.HandleFunc("/pause-loop", server.pauseLoop)
	mux.HandleFunc("/no-range-slow", server.noRangeSlow)
	mux.HandleFunc("/mutable", server.mutable)
	mux.HandleFunc("/interrupt", server.interrupt)
	mux.HandleFunc("/range-reset", server.rangeReset)
	mux.HandleFunc("/range-slow", server.rangeSlow)
	mux.HandleFunc("/range-429", server.retryingRange(http.StatusTooManyRequests))
	mux.HandleFunc("/range-500", server.retryingRange(http.StatusInternalServerError))
	mux.HandleFunc("/range-503", server.retryingRange(http.StatusServiceUnavailable))
	mux.HandleFunc("/invalid-range", server.invalidRange)
	server.HTTP = httptest.NewServer(mux)
	return server
}

func (s *Server) Close() { s.HTTP.Close() }

func (s *Server) URL(path string) string { return s.HTTP.URL + path }

func (s *Server) SetMutableVersion(version int32) { s.mutableVersion.Store(version) }

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="fixture.bin"`)
	w.Header().Set("ETag", `"fixture-v1"`)
	w.Header().Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
	w.Header().Set("Accept-Ranges", "bytes")
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len(s.Payload)))
		return
	}
	if r.Header.Get("Range") == "bytes=0-0" {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(s.Payload)))
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(s.Payload[:1])
		return
	}
	if start, end, ranged := byteRange(r.Header.Get("Range"), int64(len(s.Payload))); ranged && start < int64(len(s.Payload)) {
		if ifRange := r.Header.Get("If-Range"); ifRange != "" && ifRange != `"fixture-v1"` && ifRange != "Wed, 01 Jan 2025 00:00:00 GMT" {
			w.Header().Set("Content-Length", strconv.Itoa(len(s.Payload)))
			_, _ = w.Write(s.Payload)
			return
		}
		remaining := s.Payload[start : end+1]
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(s.Payload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(remaining)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(remaining)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(s.Payload)))
	_, _ = w.Write(s.Payload)
}

func byteRange(value string, total int64) (int64, int64, bool) {
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(parts) != 2 || parts[0] == "" {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= total {
		return 0, 0, false
	}
	end := total - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || end >= total {
			return 0, 0, false
		}
	}
	return start, end, true
}

func (s *Server) unknown(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	_, _ = w.Write(s.Payload)
}

func (s *Server) slow(w http.ResponseWriter, r *http.Request) {
	const total = 32 * 1024 * 1024
	s.rangedZeroStream(w, r, total, "", 32*1024, 2*time.Millisecond)
}

func (s *Server) pauseLoop(w http.ResponseWriter, r *http.Request) {
	const total = 1024 * 1024 * 1024
	s.rangedZeroStream(w, r, total, `"pause-loop-v1"`, 8*1024, time.Millisecond)
}

func (s *Server) mutable(w http.ResponseWriter, r *http.Request) {
	const total = 32 * 1024 * 1024
	etag := fmt.Sprintf(`"mutable-v%d"`, s.mutableVersion.Load())
	s.rangedZeroStream(w, r, total, etag, 32*1024, 2*time.Millisecond)
}

func (s *Server) noRangeSlow(w http.ResponseWriter, r *http.Request) {
	const total = 32 * 1024 * 1024
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(total))
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(total))
	streamZeros(w, r, total, 32*1024, 2*time.Millisecond)
}

func (s *Server) rangedZeroStream(w http.ResponseWriter, r *http.Request, total int, etag string, chunkSize int, delay time.Duration) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(total))
		return
	}
	if r.Header.Get("Range") == "bytes=0-0" {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", total))
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
		return
	}
	start := int64(0)
	parsed, end, ranged := byteRange(r.Header.Get("Range"), int64(total))
	if ranged {
		start = parsed
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	}
	remaining := int64(total) - start
	if ranged {
		remaining = end - start + 1
	}
	w.Header().Set("Content-Length", strconv.FormatInt(remaining, 10))
	if ranged {
		w.WriteHeader(http.StatusPartialContent)
	}
	streamZeros(w, r, remaining, chunkSize, delay)
}

func streamZeros(w http.ResponseWriter, r *http.Request, total int64, chunkSize int, delay time.Duration) {
	chunk := make([]byte, chunkSize)
	for written := int64(0); written < total; written += int64(len(chunk)) {
		toWrite := chunk
		if remaining := total - written; remaining < int64(len(chunk)) {
			toWrite = chunk[:remaining]
		}
		select {
		case <-r.Context().Done():
			return
		default:
		}
		if _, err := w.Write(toWrite); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(delay)
	}
}

func (s *Server) interrupt(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", "100")
		return
	}
	if r.Header.Get("Range") == "bytes=0-0" {
		w.Header().Set("Content-Range", "bytes 0-0/100")
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
		return
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return
	}
	writeInterruptedResponse(connection, buffer)
}

func writeInterruptedResponse(connection net.Conn, buffer *bufio.ReadWriter) {
	defer connection.Close()
	_, _ = buffer.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100\r\nContent-Type: application/octet-stream\r\n\r\nshort")
	_ = buffer.Flush()
}

func (s *Server) rangeSlow(w http.ResponseWriter, r *http.Request) {
	if start, _, ranged := byteRange(r.Header.Get("Range"), int64(len(s.Payload))); ranged && r.Header.Get("Range") != "bytes=0-0" && start > 0 {
		time.Sleep(40 * time.Millisecond)
	}
	s.file(w, r)
}

func (s *Server) retryingRange(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead && r.Header.Get("Range") != "bytes=0-0" {
			key := fmt.Sprintf("%s:%s", r.URL.Path, r.Header.Get("Range"))
			if s.incrementRequest(key) == 1 {
				w.WriteHeader(status)
				return
			}
		}
		s.file(w, r)
	}
}

func (s *Server) invalidRange(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead || r.Header.Get("Range") == "bytes=0-0" || r.Header.Get("Range") == "" {
		s.file(w, r)
		return
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(s.Payload)))
	w.Header().Set("Content-Length", "1")
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(s.Payload[:1])
}

func (s *Server) rangeReset(w http.ResponseWriter, r *http.Request) {
	start, end, ranged := byteRange(r.Header.Get("Range"), int64(len(s.Payload)))
	if r.Method == http.MethodHead || r.Header.Get("Range") == "bytes=0-0" || !ranged {
		s.file(w, r)
		return
	}
	key := "reset:" + strconv.FormatInt(end, 10)
	if s.incrementRequest(key) > 1 {
		s.file(w, r)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
		return
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	length := end - start + 1
	partial := length / 2
	if partial == 0 {
		partial = 1
	}
	_, _ = buffer.WriteString(fmt.Sprintf("HTTP/1.1 206 Partial Content\r\nContent-Length: %d\r\nContent-Range: bytes %d-%d/%d\r\n\r\n", length, start, end, len(s.Payload)))
	_, _ = buffer.Write(s.Payload[start : start+partial])
	_ = buffer.Flush()
}

func (s *Server) incrementRequest(key string) int {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	s.requestCounts[key]++
	return s.requestCounts[key]
}
