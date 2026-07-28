package download

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("download not found")

type Repository interface {
	Create(context.Context, Download) error
	Get(context.Context, string) (Download, error)
	List(context.Context) ([]Download, error)
	Save(context.Context, Download) error
	Delete(context.Context, string) error
}

type Download struct {
	ID              string
	URL             string
	FinalURL        string
	FileName        string
	DestinationPath string
	TempPath        string
	State           State
	TotalBytes      int64
	DownloadedBytes int64
	RangeSupported  bool
	ETag            string
	LastModified    string
	MIMEType        string
	CreatedAt       time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	LastError       string
	RetryCount      int
	Connections     int
	BandwidthLimit  int64
	// QueueBandwidthLimit is resolved from the current queue policy immediately
	// before a run. It is intentionally not persisted on the download record.
	QueueBandwidthLimit int64
	CategoryID          string
	QueueID             string
	QueuePosition       int64
	Priority            int
	SiteProfileID       string
	RequestOptions      RequestOptions
	Segments            []Segment
}

type RequestOptions struct {
	Headers       map[string]string
	ProxyURL      string
	ProxyUsername string
	ProxyPassword string
}

type SegmentState string

const (
	SegmentPending     SegmentState = "pending"
	SegmentDownloading SegmentState = "downloading"
	SegmentPaused      SegmentState = "paused"
	SegmentCompleted   SegmentState = "completed"
	SegmentFailed      SegmentState = "failed"
)

type Segment struct {
	ID          string
	DownloadID  string
	Index       int
	StartByte   int64
	EndByte     int64
	CurrentByte int64
	State       SegmentState
	RetryCount  int
	TempPath    string
	LastError   string
}

type ProbeResult struct {
	URL            string
	FinalURL       string
	FileName       string
	TotalBytes     int64
	MIMEType       string
	ETag           string
	LastModified   string
	RangeSupported bool
}

type Progress struct {
	ID                  string  `json:"id"`
	DownloadedBytes     int64   `json:"downloadedBytes"`
	TotalBytes          int64   `json:"totalBytes"`
	SpeedBytesPerSecond float64 `json:"speedBytesPerSecond"`
	ETASeconds          int64   `json:"etaSeconds"`
	Segments            []Segment
}

func (d Download) CompletedSegmentBytes() int64 {
	var total int64
	for _, segment := range d.Segments {
		if segment.CurrentByte > segment.StartByte {
			total += segment.CurrentByte - segment.StartByte
		}
	}
	return total
}
