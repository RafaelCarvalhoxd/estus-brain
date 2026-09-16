package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNoteValidate(t *testing.T) {
	ok := []Note{
		{},
		{Title: "Estudo"},
		{Title: "Aula", Body: "texto", Content: json.RawMessage(`{"type":"doc","content":[]}`)},
		{Content: json.RawMessage(`null`)},
	}
	for i, n := range ok {
		if err := n.Validate(); err != nil {
			t.Fatalf("case %d rejected: %v", i, err)
		}
	}

	bad := []Note{
		{Title: strings.Repeat("a", MaxNoteTitleChars+1)},
		{Content: json.RawMessage(`[1,2]`)},
		{Content: json.RawMessage(`{"type":`)},
		{Content: json.RawMessage(`{"x":"` + strings.Repeat("a", MaxNoteContentBytes) + `"}`)},
	}
	for i, n := range bad {
		if err := n.Validate(); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}
