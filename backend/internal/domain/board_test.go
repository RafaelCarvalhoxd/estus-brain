package domain

import (
	"strings"
	"testing"
)

func TestValidateBoardName(t *testing.T) {
	if err := ValidateBoardName("Fluxo de onboarding"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 121)} {
		if err := ValidateBoardName(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestValidateBoardScene(t *testing.T) {
	for _, ok := range []string{`{}`, `{"elements":[],"files":{}}`, "  {\"elements\":[{\"id\":\"a\"}]}\n"} {
		if err := ValidateBoardScene([]byte(ok)); err != nil {
			t.Fatalf("valid scene %q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{``, `[]`, `"x"`, `{"elements":`, `null`} {
		if err := ValidateBoardScene([]byte(bad)); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	huge := []byte(`{"x":"` + strings.Repeat("a", MaxBoardSceneBytes) + `"}`)
	if err := ValidateBoardScene(huge); err == nil {
		t.Fatal("oversized scene accepted")
	}
}

func TestValidateBoardPreview(t *testing.T) {
	if err := ValidateBoardPreview(""); err != nil {
		t.Fatalf("empty preview rejected: %v", err)
	}
	if err := ValidateBoardPreview(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`); err != nil {
		t.Fatalf("svg preview rejected: %v", err)
	}
	if err := ValidateBoardPreview(`<script>alert(1)</script>`); err == nil {
		t.Fatal("non-svg preview accepted")
	}
	if err := ValidateBoardPreview("<svg>" + strings.Repeat("a", MaxBoardPreviewBytes) + "</svg>"); err == nil {
		t.Fatal("oversized preview accepted")
	}
}
