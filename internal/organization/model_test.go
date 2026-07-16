package organization

import "testing"

func TestMatchCategoryIsDeterministic(t *testing.T) {
	categories := []Category{
		{ID: "z", Extensions: []string{".ZIP"}, Priority: 5},
		{ID: "a", Extensions: []string{"zip"}, Priority: 5},
		{ID: "low", Extensions: []string{"zip"}, Priority: 1},
	}
	match := MatchCategory("Archive.ZIP", categories)
	if match == nil || match.ID != "a" {
		t.Fatalf("expected stable ID tie-break, got %#v", match)
	}
}

func TestNormalizeExtensions(t *testing.T) {
	got := NormalizeExtensions([]string{" .JPG ", "jpg", "png", "bad/path"})
	if len(got) != 2 || got[0] != "jpg" || got[1] != "png" {
		t.Fatalf("unexpected normalized extensions: %#v", got)
	}
}
