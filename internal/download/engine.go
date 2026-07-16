package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/fluxdm/fluxdm/internal/security"
)

var ErrCancelled = errors.New("download cancelled")
var ErrRangeUnsupported = errors.New("server did not honor the resume range")
var ErrRangeUnreliable = errors.New("server returned an unreliable range response")

const maxSegmentAttempts = 4

var fullContentRangePattern = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+)$`)

type Engine struct {
	client           *http.Client
	bufferSize       int
	progressInterval time.Duration
	retryBaseDelay   time.Duration
	dynamicSplitMin  int64
	slowThreshold    time.Duration
	globalLimiter    *bandwidthLimiter
	queueLimitersMu  sync.Mutex
	queueLimiters    map[string]*bandwidthLimiter
}

func NewEngine(client *http.Client) *Engine {
	return NewEngineWithOptions(client, EngineOptions{})
}

type EngineOptions struct {
	BufferSize           int
	ProgressInterval     time.Duration
	RetryBaseDelay       time.Duration
	DynamicSplitMinBytes int64
	SlowSegmentThreshold time.Duration
}

func NewEngineWithOptions(client *http.Client, options EngineOptions) *Engine {
	if options.BufferSize <= 0 {
		options.BufferSize = 256 * 1024
	}
	if options.ProgressInterval <= 0 {
		options.ProgressInterval = 250 * time.Millisecond
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = 75 * time.Millisecond
	}
	if options.DynamicSplitMinBytes <= 0 {
		options.DynamicSplitMinBytes = 4 * 1024 * 1024
	}
	if options.SlowSegmentThreshold <= 0 {
		options.SlowSegmentThreshold = 300 * time.Millisecond
	}
	return &Engine{
		client: client, bufferSize: options.BufferSize, progressInterval: options.ProgressInterval,
		retryBaseDelay: options.RetryBaseDelay, dynamicSplitMin: options.DynamicSplitMinBytes,
		slowThreshold: options.SlowSegmentThreshold, globalLimiter: newBandwidthLimiter(0),
		queueLimiters: make(map[string]*bandwidthLimiter),
	}
}

func (e *Engine) SetGlobalBandwidthLimit(bytesPerSecond int64) error {
	if bytesPerSecond < 0 {
		return fmt.Errorf("bandwidth limit cannot be negative")
	}
	e.globalLimiter.setRate(bytesPerSecond)
	return nil
}

func (e *Engine) Download(ctx context.Context, task Download, onProgress func(Progress)) error {
	requestURL := task.FinalURL
	if requestURL == "" {
		requestURL = task.URL
	}
	if _, err := security.ValidateHTTPURL(requestURL); err != nil {
		return err
	}
	if task.TempPath == "" || task.DestinationPath == "" {
		return fmt.Errorf("download paths are required")
	}
	if len(task.Segments) == 0 {
		return fmt.Errorf("download has no segments")
	}
	if task.TotalBytes >= 0 {
		if err := ValidateSegments(task.Segments, task.TotalBytes); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(task.TempPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	if task.TotalBytes >= 0 && completedBytes(task.Segments) == 0 {
		if err := file.Truncate(task.TotalBytes); err != nil {
			return fmt.Errorf("preallocate temporary file: %w", err)
		}
	} else if completedBytes(task.Segments) == 0 {
		if err := file.Truncate(0); err != nil {
			return fmt.Errorf("reset temporary file: %w", err)
		}
	}

	reporter := newSegmentReporter(task, file, e.progressInterval, onProgress)
	downloadLimiter := newBandwidthLimiter(task.BandwidthLimit)
	segmented := task.RangeSupported && task.TotalBytes > 0
	if segmented {
		err = e.downloadRanges(ctx, requestURL, task, file, reporter, downloadLimiter)
		if errors.Is(err, ErrRangeUnreliable) && completedBytes(task.Segments) == 0 {
			err = e.fallbackToSingle(ctx, requestURL, &task, file, reporter, downloadLimiter)
		}
	} else {
		err = e.downloadFullWithRetry(ctx, requestURL, &task, file, reporter, downloadLimiter)
	}
	if err != nil {
		_ = reporter.report(true)
		return err
	}
	if err := reporter.report(true); err != nil {
		return err
	}
	if task.TotalBytes >= 0 {
		info, statErr := file.Stat()
		if statErr != nil {
			return fmt.Errorf("inspect completed download: %w", statErr)
		}
		if info.Size() != task.TotalBytes || reporter.downloaded() != task.TotalBytes {
			return fmt.Errorf("download size mismatch: received %d of %d bytes", reporter.downloaded(), task.TotalBytes)
		}
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("flush temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	closed = true
	if err := os.Rename(task.TempPath, task.DestinationPath); err != nil {
		return fmt.Errorf("complete download: %w", err)
	}
	return nil
}

func (e *Engine) downloadRanges(ctx context.Context, requestURL string, task Download, file *os.File, reporter *segmentReporter, downloadLimiter *bandwidthLimiter) error {
	workerCount := len(task.Segments)
	if workerCount > 16 {
		workerCount = 16
	}
	controller := newConnectionController(workerCount)
	type segmentJob struct {
		index int
		ctx   context.Context
	}
	type segmentResult struct {
		index int
		err   error
	}
	type activeSegment struct {
		cancel  context.CancelFunc
		started time.Time
	}
	jobs := make(chan segmentJob, workerCount)
	results := make(chan segmentResult, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				err := e.downloadSegmentWithRetry(job.ctx, requestURL, task, job.index, file, reporter, downloadLimiter, controller)
				results <- segmentResult{index: job.index, err: err}
			}
		}()
	}
	active := make(map[int]activeSegment, workerCount)
	splitRequested := make(map[int]bool)
	start := func(index int) {
		segmentCtx, cancel := context.WithCancel(ctx)
		active[index] = activeSegment{cancel: cancel, started: time.Now()}
		jobs <- segmentJob{index: index, ctx: segmentCtx}
	}
	for index := range task.Segments {
		start(index)
	}
	tickDuration := e.slowThreshold / 2
	if tickDuration <= 0 || tickDuration > 100*time.Millisecond {
		tickDuration = 100 * time.Millisecond
	}
	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()
	var firstErr error
	for len(active) > 0 {
		select {
		case <-ctx.Done():
			if firstErr == nil {
				firstErr = ErrCancelled
			}
			for _, running := range active {
				running.cancel()
			}
		case result := <-results:
			running, exists := active[result.index]
			if !exists {
				continue
			}
			running.cancel()
			delete(active, result.index)
			if splitRequested[result.index] && errors.Is(result.err, ErrCancelled) && ctx.Err() == nil {
				delete(splitRequested, result.index)
				newIndex, ok := reporter.splitTail(result.index, e.dynamicSplitMin, task.ID, task.TempPath)
				if ok {
					_ = reporter.report(true)
					start(result.index)
					start(newIndex)
					continue
				}
			}
			if result.err != nil && !errors.Is(result.err, ErrCancelled) && firstErr == nil {
				firstErr = result.err
				for _, other := range active {
					other.cancel()
				}
			}
		case <-ticker.C:
			if firstErr != nil || len(active) >= workerCount || reporter.segmentCount() >= workerCount*4 {
				continue
			}
			candidate := reporter.segmentCount() - 1
			running, ok := active[candidate]
			if !ok || splitRequested[candidate] || time.Since(running.started) < e.slowThreshold {
				continue
			}
			if reporter.remaining(candidate) >= 2*e.dynamicSplitMin {
				splitRequested[candidate] = true
				running.cancel()
			}
		}
	}
	close(jobs)
	workers.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (e *Engine) downloadSegmentWithRetry(ctx context.Context, requestURL string, task Download, index int, file *os.File, reporter *segmentReporter, downloadLimiter *bandwidthLimiter, controller *connectionController) error {
	for attempt := 0; attempt < maxSegmentAttempts; attempt++ {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		reporter.markState(index, SegmentDownloading, attempt, "")
		if err := controller.acquire(ctx); err != nil {
			return ErrCancelled
		}
		err := e.downloadSegment(ctx, requestURL, task, index, file, reporter, downloadLimiter)
		controller.release()
		if err == nil {
			controller.success()
			reporter.markState(index, SegmentCompleted, attempt, "")
			return nil
		}
		if ctx.Err() != nil {
			return ErrCancelled
		}
		if errors.Is(err, ErrRangeUnreliable) || !retryable(err) {
			reporter.markState(index, SegmentFailed, attempt, err.Error())
			return err
		}
		var status *HTTPStatusError
		if errors.As(err, &status) && (status.StatusCode == http.StatusTooManyRequests || status.StatusCode == http.StatusServiceUnavailable) {
			controller.overload()
		}
		reporter.markState(index, SegmentPending, attempt+1, err.Error())
		if err := sleepContext(ctx, retryDelay(e.retryBaseDelay, index, attempt)); err != nil {
			return ErrCancelled
		}
	}
	err := fmt.Errorf("segment %d exhausted retries", index)
	reporter.markState(index, SegmentFailed, maxSegmentAttempts-1, err.Error())
	return err
}

func (e *Engine) downloadSegment(ctx context.Context, requestURL string, task Download, index int, file *os.File, reporter *segmentReporter, downloadLimiter *bandwidthLimiter) error {
	segment := reporter.segment(index)
	if segment.CurrentByte == segment.EndByte+1 {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create segment request: %w", err)
	}
	applyRequestOptions(request, task.RequestOptions)
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", segment.CurrentByte, segment.EndByte))
	setIfRange(request, task)
	client, err := clientForOptions(e.client, task.RequestOptions)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		return fmt.Errorf("start segment %d: %w", index, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return fmt.Errorf("%w: %w: segment %d received 200", ErrRangeUnreliable, ErrRangeUnsupported, index)
	}
	if response.StatusCode != http.StatusPartialContent {
		return &HTTPStatusError{StatusCode: response.StatusCode, Status: response.Status}
	}
	expectedLength := segment.EndByte - segment.CurrentByte + 1
	if err := validateSegmentContentRange(response.Header.Get("Content-Range"), segment.CurrentByte, segment.EndByte, task.TotalBytes); err != nil {
		return err
	}
	if response.ContentLength >= 0 && response.ContentLength != expectedLength {
		return fmt.Errorf("%w: segment %d length %d, expected %d", ErrRangeUnreliable, index, response.ContentLength, expectedLength)
	}
	if task.TotalBytes >= 0 {
		if info, statErr := file.Stat(); statErr != nil {
			return fmt.Errorf("inspect preallocated file: %w", statErr)
		} else if info.Size() != task.TotalBytes {
			if truncateErr := file.Truncate(task.TotalBytes); truncateErr != nil {
				return fmt.Errorf("preallocate temporary file: %w", truncateErr)
			}
		}
	}
	writer := &segmentWriter{file: file, offset: segment.CurrentByte, end: segment.EndByte + 1, ctx: ctx,
		limiters: e.limitersFor(task, downloadLimiter), onWrite: func(next int64) error { return reporter.advance(index, next) }}
	written, copyErr := io.CopyBuffer(writer, io.LimitReader(response.Body, expectedLength+1), make([]byte, e.bufferSize))
	if copyErr != nil {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		return fmt.Errorf("stream segment %d: %w", index, copyErr)
	}
	if written != expectedLength {
		return fmt.Errorf("segment %d ended early: received %d of %d bytes", index, written, expectedLength)
	}
	return nil
}

func (e *Engine) fallbackToSingle(ctx context.Context, requestURL string, task *Download, file *os.File, reporter *segmentReporter, downloadLimiter *bandwidthLimiter) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("reset unreliable ranged download: %w", err)
	}
	end := task.TotalBytes - 1
	task.Segments = []Segment{{ID: task.ID + ":0", DownloadID: task.ID, Index: 0, StartByte: 0, EndByte: end, CurrentByte: 0, State: SegmentPending, TempPath: task.TempPath}}
	task.Connections = 1
	reporter.replace(task.Segments)
	return e.downloadFullWithRetry(ctx, requestURL, task, file, reporter, downloadLimiter)
}

func (e *Engine) downloadFullWithRetry(ctx context.Context, requestURL string, task *Download, file *os.File, reporter *segmentReporter, downloadLimiter *bandwidthLimiter) error {
	for attempt := 0; attempt < maxSegmentAttempts; attempt++ {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		if err := file.Truncate(0); err != nil {
			return fmt.Errorf("reset full download: %w", err)
		}
		if task.TotalBytes >= 0 {
			if err := file.Truncate(task.TotalBytes); err != nil {
				return fmt.Errorf("preallocate full download: %w", err)
			}
		}
		reporter.resetSingle(attempt)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return fmt.Errorf("create download request: %w", err)
		}
		applyRequestOptions(request, task.RequestOptions)
		client, clientErr := clientForOptions(e.client, task.RequestOptions)
		if clientErr != nil {
			return clientErr
		}
		response, err := client.Do(request)
		if err == nil && !isSuccess(response.StatusCode) {
			err = &HTTPStatusError{StatusCode: response.StatusCode, Status: response.Status}
		}
		if err == nil {
			writer := &segmentWriter{file: file, offset: 0, end: task.TotalBytes, ctx: ctx,
				limiters: e.limitersFor(*task, downloadLimiter), onWrite: func(next int64) error { return reporter.advance(0, next) }}
			if task.TotalBytes < 0 {
				writer.end = -1
			}
			written, copyErr := io.CopyBuffer(writer, response.Body, make([]byte, e.bufferSize))
			closeErr := response.Body.Close()
			if copyErr == nil {
				copyErr = closeErr
			}
			if copyErr == nil && response.ContentLength >= 0 && written != response.ContentLength {
				copyErr = fmt.Errorf("download ended early: received %d of %d bytes", written, response.ContentLength)
			}
			if copyErr == nil && task.TotalBytes >= 0 && written != task.TotalBytes {
				copyErr = fmt.Errorf("download size mismatch: received %d of %d bytes", written, task.TotalBytes)
			}
			if copyErr == nil {
				reporter.markState(0, SegmentCompleted, attempt, "")
				return nil
			}
			err = copyErr
		} else if response != nil {
			response.Body.Close()
		}
		if ctx.Err() != nil {
			return ErrCancelled
		}
		if !retryable(err) {
			reporter.markState(0, SegmentFailed, attempt, err.Error())
			return err
		}
		reporter.markState(0, SegmentPending, attempt+1, err.Error())
		if sleepContext(ctx, retryDelay(e.retryBaseDelay, 0, attempt)) != nil {
			return ErrCancelled
		}
	}
	return fmt.Errorf("download exhausted retries")
}

type segmentWriter struct {
	file     *os.File
	offset   int64
	end      int64
	ctx      context.Context
	limiters []*bandwidthLimiter
	onWrite  func(int64) error
}

func (w *segmentWriter) Write(data []byte) (int, error) {
	if w.end >= 0 && w.offset+int64(len(data)) > w.end {
		return 0, fmt.Errorf("segment write exceeds assigned range")
	}
	for _, limiter := range w.limiters {
		if limiter != nil {
			if err := limiter.wait(w.ctx, len(data)); err != nil {
				return 0, err
			}
		}
	}
	n, err := w.file.WriteAt(data, w.offset)
	w.offset += int64(n)
	if err == nil && w.onWrite != nil {
		err = w.onWrite(w.offset)
	}
	return n, err
}

type segmentReporter struct {
	mu             sync.Mutex
	id             string
	total          int64
	segments       []Segment
	file           *os.File
	interval       time.Duration
	lastAt         time.Time
	lastSpeedAt    time.Time
	lastSpeedBytes int64
	smoothedSpeed  float64
	callback       func(Progress)
}

func newSegmentReporter(task Download, file *os.File, interval time.Duration, callback func(Progress)) *segmentReporter {
	now := time.Now()
	return &segmentReporter{
		id: task.ID, total: task.TotalBytes, segments: cloneSegments(task.Segments), file: file,
		interval: interval, lastAt: now, lastSpeedAt: now,
		lastSpeedBytes: completedBytes(task.Segments), callback: callback,
	}
}

func (r *segmentReporter) segment(index int) Segment {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.segments[index]
}
func (r *segmentReporter) segmentCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.segments)
}
func (r *segmentReporter) remaining(index int) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	segment := r.segments[index]
	return segment.EndByte - segment.CurrentByte + 1
}
func (r *segmentReporter) splitTail(index int, minimum int64, downloadID, tempPath string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index != len(r.segments)-1 {
		return 0, false
	}
	segment := &r.segments[index]
	remaining := segment.EndByte - segment.CurrentByte + 1
	if remaining < 2*minimum {
		return 0, false
	}
	oldEnd := segment.EndByte
	midpoint := segment.CurrentByte + remaining/2
	segment.EndByte = midpoint - 1
	segment.State = SegmentPending
	segment.LastError = ""
	newIndex := len(r.segments)
	r.segments = append(r.segments, Segment{
		ID: fmt.Sprintf("%s:%d", downloadID, newIndex), DownloadID: downloadID, Index: newIndex,
		StartByte: midpoint, EndByte: oldEnd, CurrentByte: midpoint,
		State: SegmentPending, TempPath: tempPath,
	})
	return newIndex, true
}
func (r *segmentReporter) advance(index int, next int64) error {
	r.mu.Lock()
	segment := &r.segments[index]
	if next < segment.CurrentByte || (segment.EndByte >= segment.StartByte && next > segment.EndByte+1) {
		r.mu.Unlock()
		return fmt.Errorf("segment %d progress is out of range", index)
	}
	segment.CurrentByte = next
	r.mu.Unlock()
	return r.report(false)
}
func (r *segmentReporter) markState(index int, state SegmentState, retries int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	segment := &r.segments[index]
	segment.State = state
	segment.RetryCount = retries
	segment.LastError = message
}
func (r *segmentReporter) replace(segments []Segment) {
	r.mu.Lock()
	r.segments = cloneSegments(segments)
	r.lastAt = time.Time{}
	r.lastSpeedAt = time.Now()
	r.lastSpeedBytes = completedBytes(segments)
	r.smoothedSpeed = 0
	r.mu.Unlock()
}
func (r *segmentReporter) resetSingle(retries int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.segments[0].CurrentByte = r.segments[0].StartByte
	r.segments[0].State = SegmentDownloading
	r.segments[0].RetryCount = retries
	r.segments[0].LastError = ""
}
func (r *segmentReporter) downloaded() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return completedBytes(r.segments)
}
func (r *segmentReporter) report(force bool) error {
	r.mu.Lock()
	if !force && time.Since(r.lastAt) < r.interval {
		r.mu.Unlock()
		return nil
	}
	if err := r.file.Sync(); err != nil {
		r.mu.Unlock()
		return err
	}
	progress := Progress{ID: r.id, DownloadedBytes: completedBytes(r.segments), TotalBytes: r.total, Segments: cloneSegments(r.segments)}
	now := time.Now()
	if elapsed := now.Sub(r.lastSpeedAt).Seconds(); elapsed > 0 {
		instant := float64(progress.DownloadedBytes-r.lastSpeedBytes) / elapsed
		if instant >= 0 {
			if r.smoothedSpeed == 0 {
				r.smoothedSpeed = instant
			} else {
				r.smoothedSpeed = 0.25*instant + 0.75*r.smoothedSpeed
			}
		}
	}
	progress.SpeedBytesPerSecond = r.smoothedSpeed
	progress.ETASeconds = -1
	if r.total >= progress.DownloadedBytes && r.smoothedSpeed > 0 {
		progress.ETASeconds = int64(float64(r.total-progress.DownloadedBytes)/r.smoothedSpeed + 0.999)
	}
	r.lastSpeedAt = now
	r.lastSpeedBytes = progress.DownloadedBytes
	r.lastAt = now
	callback := r.callback
	r.mu.Unlock()
	if callback != nil {
		callback(progress)
	}
	return nil
}

func validateSegmentContentRange(value string, expectedStart, expectedEnd, expectedTotal int64) error {
	matches := fullContentRangePattern.FindStringSubmatch(value)
	if len(matches) != 4 {
		return fmt.Errorf("%w: invalid Content-Range %q", ErrRangeUnreliable, value)
	}
	start, err1 := strconv.ParseInt(matches[1], 10, 64)
	end, err2 := strconv.ParseInt(matches[2], 10, 64)
	total, err3 := strconv.ParseInt(matches[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || start != expectedStart || end != expectedEnd || (expectedTotal >= 0 && total != expectedTotal) {
		return fmt.Errorf("%w: unexpected Content-Range %q", ErrRangeUnreliable, value)
	}
	return nil
}

func validateResumeContentRange(value string, expectedStart, expectedTotal int64) error {
	matches := fullContentRangePattern.FindStringSubmatch(value)
	if len(matches) != 4 {
		return fmt.Errorf("invalid resume Content-Range %q", value)
	}
	start, _ := strconv.ParseInt(matches[1], 10, 64)
	end, _ := strconv.ParseInt(matches[2], 10, 64)
	total, _ := strconv.ParseInt(matches[3], 10, 64)
	if start != expectedStart || end < start || (expectedTotal >= 0 && total != expectedTotal) {
		return fmt.Errorf("unexpected resume Content-Range %q", value)
	}
	return nil
}

func setIfRange(request *http.Request, task Download) {
	if task.ETag != "" {
		request.Header.Set("If-Range", task.ETag)
	} else if task.LastModified != "" {
		request.Header.Set("If-Range", task.LastModified)
	}
}
func retryable(err error) bool {
	var status *HTTPStatusError
	if errors.As(err, &status) {
		return status.StatusCode == http.StatusTooManyRequests || status.StatusCode == http.StatusInternalServerError || status.StatusCode == http.StatusServiceUnavailable
	}
	return !errors.Is(err, ErrRangeUnreliable) && !errors.Is(err, ErrCancelled)
}
func retryDelay(base time.Duration, segment, attempt int) time.Duration {
	multiplier := 1 << attempt
	jitterPercent := ((segment+1)*37+(attempt+1)*17)%41 - 20
	return time.Duration(int64(base) * int64(multiplier) * int64(100+jitterPercent) / 100)
}
func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func cloneSegments(value []Segment) []Segment { return append([]Segment(nil), value...) }
func completedBytes(segments []Segment) int64 {
	var total int64
	for _, segment := range segments {
		if segment.CurrentByte > segment.StartByte {
			total += segment.CurrentByte - segment.StartByte
		}
	}
	return total
}

type bandwidthLimiter struct {
	mu   sync.Mutex
	rate int64
	next time.Time
}

type connectionController struct {
	mu            sync.Mutex
	desired       int
	limit         int
	active        int
	successStreak int
	notify        chan struct{}
}

func newConnectionController(desired int) *connectionController {
	return &connectionController{desired: desired, limit: desired, notify: make(chan struct{}, 1)}
}

func (c *connectionController) acquire(ctx context.Context) error {
	for {
		c.mu.Lock()
		if c.active < c.limit {
			c.active++
			hasCapacity := c.active < c.limit
			c.mu.Unlock()
			if hasCapacity {
				c.signal()
			}
			return nil
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.notify:
		}
	}
}

func (c *connectionController) release() {
	c.mu.Lock()
	if c.active > 0 {
		c.active--
	}
	c.mu.Unlock()
	c.signal()
}

func (c *connectionController) overload() {
	c.mu.Lock()
	if c.limit > 1 {
		c.limit = max(1, c.limit/2)
	}
	c.successStreak = 0
	c.mu.Unlock()
	c.signal()
}

func (c *connectionController) success() {
	c.mu.Lock()
	if c.limit < c.desired {
		c.successStreak++
		if c.successStreak >= c.limit {
			c.limit++
			c.successStreak = 0
		}
	}
	c.mu.Unlock()
	c.signal()
}

func (c *connectionController) signal() {
	select {
	case c.notify <- struct{}{}:
	default:
	}
}

func newBandwidthLimiter(rate int64) *bandwidthLimiter { return &bandwidthLimiter{rate: rate} }

func (e *Engine) limitersFor(task Download, perDownload *bandwidthLimiter) []*bandwidthLimiter {
	limiters := []*bandwidthLimiter{perDownload}
	if task.QueueID != "" {
		e.queueLimitersMu.Lock()
		queueLimiter := e.queueLimiters[task.QueueID]
		if queueLimiter == nil {
			queueLimiter = newBandwidthLimiter(task.QueueBandwidthLimit)
			e.queueLimiters[task.QueueID] = queueLimiter
		} else {
			queueLimiter.setRateIfChanged(task.QueueBandwidthLimit)
		}
		e.queueLimitersMu.Unlock()
		limiters = append(limiters, queueLimiter)
	}
	return append(limiters, e.globalLimiter)
}

func (l *bandwidthLimiter) setRate(rate int64) {
	l.mu.Lock()
	l.rate = rate
	l.next = time.Time{}
	l.mu.Unlock()
}

func (l *bandwidthLimiter) setRateIfChanged(rate int64) {
	l.mu.Lock()
	if l.rate != rate {
		l.rate = rate
		l.next = time.Time{}
	}
	l.mu.Unlock()
}

func (l *bandwidthLimiter) wait(ctx context.Context, bytes int) error {
	if bytes <= 0 {
		return nil
	}
	l.mu.Lock()
	if l.rate <= 0 {
		l.mu.Unlock()
		return nil
	}
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	duration := time.Duration(int64(time.Second) * int64(bytes) / l.rate)
	if duration <= 0 {
		duration = time.Nanosecond
	}
	l.next = l.next.Add(duration)
	target := l.next
	l.mu.Unlock()
	return sleepContext(ctx, time.Until(target))
}
