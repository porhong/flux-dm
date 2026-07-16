package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSegmentWriterRejectsOutOfRangeWrite(t *testing.T) {
	file, err := os.OpenFile(filepath.Join(t.TempDir(), "part"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := &segmentWriter{file: file, offset: 4, end: 8}
	if _, err := writer.Write(make([]byte, 5)); err == nil {
		t.Fatal("expected write beyond assigned range to fail")
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("out-of-range write changed file size to %d", info.Size())
	}
}
