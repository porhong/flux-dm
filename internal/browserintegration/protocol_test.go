package browserintegration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeMessageRoundTripAndValidation(t *testing.T) {
	var buffer bytes.Buffer
	payload := `{"version":1,"requestId":"abc-1","type":"add","url":"https://example.test/file.zip"}`
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(len(payload)))
	buffer.WriteString(payload)
	request, err := ReadMessage(&buffer)
	if err != nil || request.URL == "" {
		t.Fatalf("request=%#v err=%v", request, err)
	}
}
func TestNativeMessageRejectsOversizedAndUnknownFields(t *testing.T) {
	var oversized bytes.Buffer
	_ = binary.Write(&oversized, binary.LittleEndian, uint32(MaxMessageBytes+1))
	if _, err := ReadMessage(&oversized); err == nil {
		t.Fatal("expected oversized rejection")
	}
	payload := `{"version":1,"requestId":"x","type":"ping","secret":"no"}`
	var unknown bytes.Buffer
	_ = binary.Write(&unknown, binary.LittleEndian, uint32(len(payload)))
	unknown.WriteString(payload)
	if _, err := ReadMessage(&unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestValidateOriginIsExact(t *testing.T) {
	if err := ValidateOrigin(ExtensionOrigin); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", ExtensionOrigin + "evil", "chrome-extension://other/"} {
		if ValidateOrigin(origin) == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
}

func TestPackagedExtensionIdentityMatchesNativeContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	manifestData, err := os.ReadFile(filepath.Join(root, "browser-extension", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
		Key     string `json:"key"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("extension version=%q", manifest.Version)
	}
	publicKey, err := base64.StdEncoding.DecodeString(manifest.Key)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(publicKey)
	var derived strings.Builder
	for _, value := range digest[:16] {
		derived.WriteByte('a' + value>>4)
		derived.WriteByte('a' + value&0x0f)
	}
	if derived.String() != ExtensionID {
		t.Fatalf("extension ID=%q, protocol=%q", derived.String(), ExtensionID)
	}

	templateData, err := os.ReadFile(filepath.Join(root, "browser-extension", "native-host", "com.fluxdm.browser.template.json"))
	if err != nil {
		t.Fatal(err)
	}
	var nativeManifest struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(templateData, &nativeManifest); err != nil {
		t.Fatal(err)
	}
	if len(nativeManifest.AllowedOrigins) != 1 || nativeManifest.AllowedOrigins[0] != ExtensionOrigin {
		t.Fatalf("native origins=%q", nativeManifest.AllowedOrigins)
	}
	installer, err := os.ReadFile(filepath.Join(root, "build", "windows", "installer", "project.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(installer, []byte(ExtensionOrigin)) {
		t.Fatal("installer native-host manifest does not contain the fixed extension origin")
	}
}
