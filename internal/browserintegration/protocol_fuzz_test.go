package browserintegration

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func FuzzReadMessage(f *testing.F) {
	f.Add([]byte(`{"version":1,"requestId":"x","type":"ping"}`))
	f.Add([]byte(`not-json`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		var input bytes.Buffer
		_ = binary.Write(&input, binary.LittleEndian, uint32(len(payload)))
		input.Write(payload)
		_, _ = ReadMessage(&input)
	})
}
