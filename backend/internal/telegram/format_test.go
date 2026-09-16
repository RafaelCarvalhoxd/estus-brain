package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToHTML(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"escapes", "a < b & c", "a &lt; b &amp; c"},
		{"bold and italic", "**Total**: *hoje*", "<b>Total</b>: <i>hoje</i>"},
		{"snake_case untouched", "use finance_list e 2 * 3 * 4", "use finance_list e 2 * 3 * 4"},
		{"code span", "rode `a<b` agora", "rode <code>a&lt;b</code> agora"},
		{"lone backtick", "um ` solto", "um ` solto"},
		{"link", "[site](https://x.com/a?b=1&c=2)", `<a href="https://x.com/a?b=1&amp;c=2">site</a>`},
		{"heading and bullets", "## Hoje\n- um\n* dois", "<b>Hoje</b>\n• um\n• dois"},
		{"fence", "```go\nx <y>\n```", "<pre>x &lt;y&gt;</pre>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToHTML(tc.in); got != tc.want {
				t.Fatalf("ToHTML(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitShortTextIsOnePart(t *testing.T) {
	if parts := Split("oi", 4000); len(parts) != 1 || parts[0] != "oi" {
		t.Fatalf("parts = %q", parts)
	}
}

func TestSplitCutsBetweenLines(t *testing.T) {
	line := strings.Repeat("a", 30)
	text := strings.Join([]string{line + "1", line + "2", line + "3"}, "\n")
	parts := Split(text, 70)
	if len(parts) < 2 {
		t.Fatalf("expected several parts, got %q", parts)
	}
	for _, p := range parts {
		if utf8.RuneCountInString(p) > 70 {
			t.Fatalf("part too long (%d): %q", utf8.RuneCountInString(p), p)
		}
	}
	if joined := strings.Join(parts, "\n"); joined != text {
		t.Fatalf("joined = %q", joined)
	}
}

func TestSplitKeepsPreBalanced(t *testing.T) {
	lines := []string{"<pre>"}
	for i := 0; i < 10; i++ {
		lines = append(lines, strings.Repeat("x", 20))
	}
	text := strings.Join(lines, "\n") + "</pre>"
	for _, p := range Split(text, 100) {
		if utf8.RuneCountInString(p) > 100 || strings.Count(p, "<pre>") != strings.Count(p, "</pre>") {
			t.Fatalf("bad part: %q", p)
		}
	}
}

func TestSplitHardCutsAVeryLongLine(t *testing.T) {
	for _, p := range Split(strings.Repeat("é", 250), 100) {
		if utf8.RuneCountInString(p) > 100 {
			t.Fatalf("part too long: %d", utf8.RuneCountInString(p))
		}
	}
}

func TestPlainText(t *testing.T) {
	if got := PlainText(`<b>a &amp; b</b> <a href="x">link</a>`); got != "a & b link" {
		t.Fatalf("PlainText = %q", got)
	}
}
