package download

import "testing"

func FuzzPlanSegmentsMaintainsRangeInvariants(f *testing.F) {
	for _, seed := range []struct {
		total    int64
		selector uint8
	}{{1, 0}, {3, 4}, {103, 3}, {1 << 20, 2}, {1<<40 + 7, 1}} {
		f.Add(seed.total, seed.selector)
	}
	connections := [...]int{1, 2, 4, 8, 16}
	f.Fuzz(func(t *testing.T, totalBytes int64, selector uint8) {
		if totalBytes <= 0 {
			t.Skip()
		}
		connectionCount := connections[int(selector)%len(connections)]
		segments, err := PlanSegments("fuzz", "part", totalBytes, connectionCount)
		if err != nil {
			t.Fatalf("plan %d bytes with %d connections: %v", totalBytes, connectionCount, err)
		}
		if err := ValidateSegments(segments, totalBytes); err != nil {
			t.Fatalf("invalid plan for %d bytes with %d connections: %v", totalBytes, connectionCount, err)
		}
		wantCount := connectionCount
		if totalBytes < int64(wantCount) {
			wantCount = int(totalBytes)
		}
		if len(segments) != wantCount {
			t.Fatalf("planned %d segments, want %d", len(segments), wantCount)
		}
		for index, segment := range segments {
			if segment.Index != index || segment.EndByte < segment.StartByte || segment.CurrentByte != segment.StartByte {
				t.Fatalf("unsafe segment %d: %+v", index, segment)
			}
			if index > 0 && segments[index-1].EndByte+1 != segment.StartByte {
				t.Fatalf("gap or overlap between segments %d and %d", index-1, index)
			}
		}
		if segments[0].StartByte != 0 || segments[len(segments)-1].EndByte != totalBytes-1 {
			t.Fatalf("plan does not cover [0,%d]", totalBytes-1)
		}
	})
}
