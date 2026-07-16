package browserintegration

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/fluxdm/fluxdm/internal/security"
)

const MaxMessageBytes = 64 * 1024
const ExtensionID = "hnemapnmnkccfommbacamppclohhcbfn"
const ExtensionOrigin = "chrome-extension://" + ExtensionID + "/"
const HostName = "com.fluxdm.browser"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type Request struct {
	Version           int    `json:"version"`
	RequestID         string `json:"requestId"`
	Type              string `json:"type"`
	URL               string `json:"url,omitempty"`
	Referrer          string `json:"referrer,omitempty"`
	SuggestedFilename string `json:"suggestedFilename,omitempty"`
	Cookies           string `json:"cookies,omitempty"`
}
type Response struct {
	Version   int    `json:"version"`
	RequestID string `json:"requestId"`
	Accepted  bool   `json:"accepted"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

func ValidateRequest(request Request) error {
	if request.Version != 1 {
		return fmt.Errorf("unsupported protocol version")
	}
	if !requestIDPattern.MatchString(request.RequestID) {
		return fmt.Errorf("invalid request ID")
	}
	switch request.Type {
	case "ping":
		return nil
	case "add":
	default:
		return fmt.Errorf("unsupported request type")
	}
	if _, err := security.ValidateHTTPURL(request.URL); err != nil {
		return err
	}
	if request.Referrer != "" {
		if _, err := security.ValidateHTTPURL(request.Referrer); err != nil {
			return fmt.Errorf("invalid referrer: %w", err)
		}
	}
	if len(request.SuggestedFilename) > 240 || strings.ContainsAny(request.SuggestedFilename, "\r\n") {
		return fmt.Errorf("invalid suggested filename")
	}
	if len(request.Cookies) > 32768 || strings.ContainsAny(request.Cookies, "\r\n") {
		return fmt.Errorf("invalid cookies")
	}
	return nil
}

func ValidateOrigin(origin string) error {
	if origin != ExtensionOrigin {
		return fmt.Errorf("untrusted extension origin")
	}
	return nil
}

func ReadMessage(reader io.Reader) (Request, error) {
	var size uint32
	if err := binary.Read(reader, binary.LittleEndian, &size); err != nil {
		return Request{}, err
	}
	if size == 0 || size > MaxMessageBytes {
		return Request{}, fmt.Errorf("native message size %d is invalid", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return Request{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("decode native message: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Request{}, fmt.Errorf("multiple or trailing JSON values")
	}
	if err := ValidateRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func WriteMessage(writer io.Writer, response Response) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if len(payload) > MaxMessageBytes {
		return fmt.Errorf("response is too large")
	}
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}
