package telegram

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	headingRe = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	bulletRe  = regexp.MustCompile(`^(\s*)[-*+]\s+(.+)$`)
	linkRe    = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)
	boldRe    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// A single *word* with no space right inside the asterisks, so "2 * 3"
	// stays as is.
	italicRe = regexp.MustCompile(`(^|[^\w*])\*([^*\s][^*]*)\*`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
)

// ToHTML turns the Markdown an engine writes into the small HTML subset
// Telegram accepts. Anything it doesn't recognize is escaped and shown as is.
func ToHTML(md string) string {
	var out, code []string
	inCode := false
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCode {
				out = append(out, "<pre>"+html.EscapeString(strings.Join(code, "\n"))+"</pre>")
				code = nil
			}
			inCode = !inCode
			continue
		}
		switch {
		case inCode:
			code = append(code, line)
		case headingRe.MatchString(line):
			out = append(out, "<b>"+inline(headingRe.FindStringSubmatch(line)[1])+"</b>")
		case bulletRe.MatchString(line):
			m := bulletRe.FindStringSubmatch(line)
			out = append(out, m[1]+"• "+inline(m[2]))
		default:
			out = append(out, inline(line))
		}
	}
	if inCode && len(code) > 0 {
		out = append(out, "<pre>"+html.EscapeString(strings.Join(code, "\n"))+"</pre>")
	}
	return strings.Join(out, "\n")
}

// inline formats one line; text between a pair of backticks is code and is
// only escaped.
func inline(line string) string {
	parts := strings.Split(line, "`")
	var b strings.Builder
	for i, p := range parts {
		switch {
		case i%2 == 1 && i < len(parts)-1:
			b.WriteString("<code>" + html.EscapeString(p) + "</code>")
		case i%2 == 1: // an unpaired backtick: keep it
			b.WriteString("`" + emphasis(p))
		default:
			b.WriteString(emphasis(p))
		}
	}
	return b.String()
}

func emphasis(s string) string {
	s = html.EscapeString(s)
	s = linkRe.ReplaceAllString(s, `<a href="$2">$1</a>`)
	s = boldRe.ReplaceAllString(s, "<b>$1</b>")
	return italicRe.ReplaceAllString(s, "$1<i>$2</i>")
}

// Split breaks a message into parts Telegram accepts (it caps a message at
// 4096 characters), cutting between lines and closing and reopening a <pre>
// block a cut lands inside.
func Split(text string, limit int) []string {
	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}
	const open, close = "<pre>", "</pre>"
	var parts []string
	cur, inPre := "", false
	for _, line := range strings.Split(text, "\n") {
		// A piece always fits a fresh part, even reopened as "<pre>\n" and
		// closed with "</pre>".
		for _, piece := range cutRunes(line, limit-len(open)-len(close)-1) {
			next := piece
			if cur != "" {
				next = cur + "\n" + piece
			}
			room := 0
			if preOpenAfter(inPre, piece) {
				room = len(close) // keep space to close the block if we cut later
			}
			if cur != "" && utf8.RuneCountInString(next)+room > limit {
				if inPre {
					cur += close
				}
				parts = append(parts, cur)
				next = piece
				if inPre {
					next = open + "\n" + piece
				}
			}
			cur = next
			inPre = preOpenAfter(inPre, piece)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

// preOpenAfter reports whether a <pre> block is still open after s.
func preOpenAfter(inPre bool, s string) bool {
	opened, closed := strings.LastIndex(s, "<pre>"), strings.LastIndex(s, "</pre>")
	switch {
	case opened > closed:
		return true
	case closed > opened:
		return false
	default:
		return inPre
	}
}

// cutRunes splits s into pieces of at most n runes ("" stays one piece).
func cutRunes(s string, n int) []string {
	r := []rune(s)
	if len(r) <= n {
		return []string{s}
	}
	var out []string
	for len(r) > n {
		out = append(out, string(r[:n]))
		r = r[n:]
	}
	return append(out, string(r))
}

// PlainText is an HTML message with its formatting dropped.
func PlainText(s string) string {
	return html.UnescapeString(tagRe.ReplaceAllString(s, ""))
}
