package logging

import (
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const redacted = "[REDACTED]"

var signedQueryPattern = regexp.MustCompile(`(?i)(token|signature|sig|key|auth|password)=([^&\s]+)`)

type Logger struct {
	logger zerolog.Logger
	file   *os.File
	mu     sync.Mutex
}

func New(path string) (*Logger, func(), error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, func() {}, err
	}
	logger := zerolog.New(file).With().Timestamp().Logger()
	return &Logger{logger: logger, file: file}, func() { _ = file.Close() }, nil
}

func NewForWriter(writer io.Writer) *Logger {
	logger := zerolog.New(writer).With().Timestamp().Logger()
	return &Logger{logger: logger}
}

func (l *Logger) Info(message string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write(l.logger.Info(), message, fields)
}

func (l *Logger) Error(message string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write(l.logger.Error(), message, fields)
}

func (l *Logger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	if err := l.file.Truncate(0); err != nil {
		return err
	}
	_, err := l.file.Seek(0, 0)
	return err
}

func (l *Logger) write(event *zerolog.Event, message string, fields map[string]any) {
	for key, value := range fields {
		if isSensitive(key) {
			event.Str(key, redacted)
			continue
		}
		event.Interface(key, value)
	}
	event.Msg(signedQueryPattern.ReplaceAllString(message, "$1="+redacted))
}

func isSensitive(key string) bool {
	key = strings.ToLower(key)
	for _, fragment := range []string{"authorization", "cookie", "password", "secret", "token", "signature", "api_key"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
}
