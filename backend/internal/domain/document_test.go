package domain

import (
	"strings"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain name is kept", "contrato aluguel.pdf", "contrato aluguel.pdf"},
		{"unix traversal keeps only the base", "../../etc/passwd", "passwd"},
		{"windows traversal keeps only the base", `..\..\boot.ini`, "boot.ini"},
		{"absolute path keeps only the base", "/var/data/nota.pdf", "nota.pdf"},
		{"control characters are dropped", "nota\x00fiscal\n.pdf", "notafiscal.pdf"},
		{"leading dots can't make a hidden file", "..env", "env"},
		{"empty falls back", "", "arquivo"},
		{"only separators falls back", "///", "arquivo"},
		{"accents survive", "apólice de seguro.pdf", "apólice de seguro.pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeFileName(tc.in); got != tc.want {
				t.Fatalf("SafeFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSafeFileNameCapsLengthKeepingExtension(t *testing.T) {
	got := SafeFileName(strings.Repeat("a", 300) + ".pdf")
	if len(got) > 120 {
		t.Fatalf("length %d, want at most 120", len(got))
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("extension lost: %q", got)
	}
}

func TestDocumentValidate(t *testing.T) {
	ok := Document{Name: "a.pdf", SizeBytes: 10}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
	for _, d := range []Document{
		{Name: "", SizeBytes: 10},
		{Name: "a.pdf", SizeBytes: 0},
		{Name: "a.pdf", SizeBytes: MaxDocumentBytes + 1},
	} {
		if err := d.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", d)
		}
	}
}

func TestDocumentFolderValidate(t *testing.T) {
	if err := (DocumentFolder{Name: "Contratos"}).Validate(); err != nil {
		t.Fatalf("valid folder rejected: %v", err)
	}
	for _, name := range []string{"", "   ", "a/b", `a\b`, strings.Repeat("x", 121)} {
		if err := (DocumentFolder{Name: name}).Validate(); err == nil {
			t.Fatalf("expected validation error for %q", name)
		}
	}
}
