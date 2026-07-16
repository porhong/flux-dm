package download

import "fmt"

var supportedConnections = map[int]struct{}{1: {}, 2: {}, 4: {}, 8: {}, 16: {}}

func ValidConnectionCount(value int) bool {
	_, ok := supportedConnections[value]
	return ok
}

// PlanSegments divides a known resource into deterministic, non-overlapping,
// inclusive ranges. Earlier ranges receive one extra byte when division is uneven.
func PlanSegments(downloadID, tempPath string, totalBytes int64, connections int) ([]Segment, error) {
	if totalBytes <= 0 {
		return nil, fmt.Errorf("segment planning requires a positive total size")
	}
	if !ValidConnectionCount(connections) {
		return nil, fmt.Errorf("unsupported connection count %d", connections)
	}
	count := connections
	if int64(count) > totalBytes {
		count = int(totalBytes)
	}
	base := totalBytes / int64(count)
	remainder := totalBytes % int64(count)
	segments := make([]Segment, 0, count)
	start := int64(0)
	for index := 0; index < count; index++ {
		length := base
		if int64(index) < remainder {
			length++
		}
		end := start + length - 1
		segments = append(segments, Segment{
			ID: downloadID + ":" + fmt.Sprint(index), DownloadID: downloadID,
			Index: index, StartByte: start, EndByte: end, CurrentByte: start,
			State: SegmentPending, TempPath: tempPath,
		})
		start = end + 1
	}
	return segments, nil
}

func ValidateSegments(segments []Segment, totalBytes int64) error {
	if len(segments) == 0 {
		return fmt.Errorf("download has no segments")
	}
	if totalBytes == 0 && len(segments) == 1 && segments[0].StartByte == 0 && segments[0].EndByte == -1 && segments[0].CurrentByte == 0 {
		return nil
	}
	expectedStart := int64(0)
	for index, segment := range segments {
		if segment.Index != index {
			return fmt.Errorf("segment index %d is out of order", segment.Index)
		}
		if segment.StartByte != expectedStart || segment.EndByte < segment.StartByte {
			return fmt.Errorf("segment %d overlaps or leaves a gap", index)
		}
		if segment.CurrentByte < segment.StartByte || segment.CurrentByte > segment.EndByte+1 {
			return fmt.Errorf("segment %d checkpoint is outside its range", index)
		}
		expectedStart = segment.EndByte + 1
	}
	if totalBytes >= 0 && expectedStart != totalBytes {
		return fmt.Errorf("segments cover %d bytes, expected %d", expectedStart, totalBytes)
	}
	return nil
}
