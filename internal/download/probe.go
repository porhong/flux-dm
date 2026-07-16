package download

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"

	fluxfs "github.com/fluxdm/fluxdm/internal/filesystem"
	"github.com/fluxdm/fluxdm/internal/security"
)

var contentRangePattern = regexp.MustCompile(`^bytes\s+0-0/(\d+)$`)

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("server returned %s", e.Status)
}

type Prober struct {
	client *http.Client
}

func NewProber(client *http.Client) *Prober {
	return &Prober{client: client}
}

func (p *Prober) Probe(ctx context.Context, rawURL string) (ProbeResult, error) {
	return p.ProbeWithOptions(ctx, rawURL, RequestOptions{})
}

func (p *Prober) ProbeWithOptions(ctx context.Context, rawURL string, options RequestOptions) (ProbeResult, error) {
	parsed, err := security.ValidateHTTPURL(rawURL)
	if err != nil {
		return ProbeResult{}, err
	}
	result := ProbeResult{URL: parsed.String(), FinalURL: parsed.String(), TotalBytes: -1}
	client, err := clientForOptions(p.client, options)
	if err != nil {
		return ProbeResult{}, err
	}

	headRequest, err := http.NewRequestWithContext(ctx, http.MethodHead, parsed.String(), nil)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("create HEAD probe: %w", err)
	}
	applyRequestOptions(headRequest, options)
	headResponse, headErr := client.Do(headRequest)
	if headErr == nil {
		defer headResponse.Body.Close()
		if isSuccess(headResponse.StatusCode) {
			mergeProbeResponse(&result, headResponse)
		} else if headResponse.StatusCode != http.StatusMethodNotAllowed && headResponse.StatusCode != http.StatusNotImplemented {
			return ProbeResult{}, &HTTPStatusError{StatusCode: headResponse.StatusCode, Status: headResponse.Status}
		}
	}

	rangeRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("create range probe: %w", err)
	}
	applyRequestOptions(rangeRequest, options)
	rangeRequest.Header.Set("Range", "bytes=0-0")
	rangeResponse, err := client.Do(rangeRequest)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("range probe: %w", err)
	}
	defer rangeResponse.Body.Close()

	switch rangeResponse.StatusCode {
	case http.StatusPartialContent:
		mergeProbeResponse(&result, rangeResponse)
		matches := contentRangePattern.FindStringSubmatch(rangeResponse.Header.Get("Content-Range"))
		if len(matches) != 2 {
			return ProbeResult{}, fmt.Errorf("server returned an invalid Content-Range")
		}
		total, parseErr := strconv.ParseInt(matches[1], 10, 64)
		if parseErr != nil {
			return ProbeResult{}, fmt.Errorf("parse Content-Range total: %w", parseErr)
		}
		result.TotalBytes = total
		result.RangeSupported = true
	case http.StatusOK:
		mergeProbeResponse(&result, rangeResponse)
		result.RangeSupported = false
	case http.StatusRequestedRangeNotSatisfiable:
		if result.TotalBytes != 0 {
			return ProbeResult{}, &HTTPStatusError{StatusCode: rangeResponse.StatusCode, Status: rangeResponse.Status}
		}
		result.RangeSupported = false
	default:
		return ProbeResult{}, &HTTPStatusError{StatusCode: rangeResponse.StatusCode, Status: rangeResponse.Status}
	}

	result.FileName = suggestedFileName(rangeResponse, result.FinalURL)
	if result.FileName == "download" && headResponse != nil {
		result.FileName = suggestedFileName(headResponse, result.FinalURL)
	}
	return result, nil
}

func mergeProbeResponse(result *ProbeResult, response *http.Response) {
	result.FinalURL = response.Request.URL.String()
	if response.ContentLength >= 0 {
		result.TotalBytes = response.ContentLength
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
			result.MIMEType = mediaType
		} else {
			result.MIMEType = contentType
		}
	}
	if value := response.Header.Get("ETag"); value != "" {
		result.ETag = value
	}
	if value := response.Header.Get("Last-Modified"); value != "" {
		result.LastModified = value
	}
}

func suggestedFileName(response *http.Response, finalURL string) string {
	if response != nil {
		if disposition := response.Header.Get("Content-Disposition"); disposition != "" {
			if _, parameters, err := mime.ParseMediaType(disposition); err == nil {
				if name := parameters["filename"]; name != "" {
					return fluxfs.SanitizeFileName(name)
				}
			}
		}
	}
	parsed, err := url.Parse(finalURL)
	if err == nil {
		if name := path.Base(parsed.Path); name != "" && name != "." && name != "/" {
			if decoded, decodeErr := url.PathUnescape(name); decodeErr == nil {
				name = decoded
			}
			return fluxfs.SanitizeFileName(name)
		}
	}
	return "download"
}

func isSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}
