package filesystem

import (
	"strings"
	"testing"
)

func FuzzSanitizeFileName(f *testing.F) {
	for _, seed := range []string{"../setup.exe", "CON", "name.txt", "a<b>c", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		name := SanitizeFileName(value)
		if name == "" || name == "." || name == ".." || len(name) > maxFileNameBytes || strings.ContainsAny(name, `<>:"/\|?*`) {
			t.Fatalf("unsafe filename %q from %q", name, value)
		}
	})
}
