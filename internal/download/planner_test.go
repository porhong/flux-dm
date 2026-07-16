package download

import (
	"strconv"
	"testing"
)

func TestPlanSegmentsProducesDeterministicContiguousRanges(t *testing.T) {
	for _, connections := range []int{1, 2, 4, 8, 16} {
		segments, err := PlanSegments("id", "file.fluxpart", 103, connections)
		if err != nil {
			t.Fatalf("plan %d connections: %v", connections, err)
		}
		if len(segments) != connections {
			t.Fatalf("segments = %d, want %d", len(segments), connections)
		}
		if err := ValidateSegments(segments, 103); err != nil {
			t.Fatalf("validate %d connections: %v", connections, err)
		}
		for index, segment := range segments {
			if segment.ID != "id:"+strconv.Itoa(index) {
				t.Fatalf("segment %d id = %q", index, segment.ID)
			}
		}
	}
}

func TestPlanSegmentsAvoidsEmptyRanges(t *testing.T) {
	segments, err := PlanSegments("id", "part", 3, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(segments))
	}
	if err := ValidateSegments(segments, 3); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSegmentsRejectsOverlapAndOutOfRangeProgress(t *testing.T) {
	tests := [][]Segment{
		{{Index: 0, StartByte: 0, EndByte: 5}, {Index: 1, StartByte: 5, EndByte: 9, CurrentByte: 5}},
		{{Index: 0, StartByte: 0, EndByte: 9, CurrentByte: 11}},
	}
	for _, segments := range tests {
		if err := ValidateSegments(segments, 10); err == nil {
			t.Fatal("expected invalid segments to be rejected")
		}
	}
}
