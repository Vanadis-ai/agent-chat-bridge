package formatter

import (
	"strings"
	"testing"
)

// --- HTML Converter Tests ---

func TestBold(t *testing.T) {
	got := ToHTML("**bold text**")
	want := "<b>bold text</b>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItalic(t *testing.T) {
	got := ToHTML("_italic text_")
	want := "<i>italic text</i>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInlineCode(t *testing.T) {
	got := ToHTML("`some code`")
	want := "<code>some code</code>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFencedCodeWithLanguage(t *testing.T) {
	input := "```go\nfmt.Println(\"hi\")\n```"
	got := ToHTML(input)
	if !strings.Contains(got, `<pre><code class="language-go">`) {
		t.Errorf("missing language class, got: %q", got)
	}
	if !strings.Contains(got, "</code></pre>") {
		t.Errorf("missing closing tags, got: %q", got)
	}
}

func TestFencedCodeWithoutLanguage(t *testing.T) {
	input := "```\nsome code\n```"
	got := ToHTML(input)
	if !strings.Contains(got, "<pre><code>") {
		t.Errorf("missing pre/code tags, got: %q", got)
	}
	if !strings.Contains(got, "some code") {
		t.Errorf("missing code content, got: %q", got)
	}
}

func TestLink(t *testing.T) {
	got := ToHTML("[text](https://example.com)")
	want := `<a href="https://example.com">text</a>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLSpecialCharsEscaped(t *testing.T) {
	got := ToHTML("Use <div> & things")
	if !strings.Contains(got, "&lt;div&gt;") {
		t.Errorf("< and > should be escaped, got: %q", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("& should be escaped, got: %q", got)
	}
}

func TestCodeBlockNoDoubleEscape(t *testing.T) {
	input := "```\nif a < b && c > d\n```"
	got := ToHTML(input)
	if strings.Contains(got, "&amp;amp;") {
		t.Errorf("double-escaped, got: %q", got)
	}
	if !strings.Contains(got, "&lt;") {
		t.Errorf("< should be escaped once, got: %q", got)
	}
	if !strings.Contains(got, "&amp;&amp;") {
		t.Errorf("&& should be escaped once, got: %q", got)
	}
}

func TestPlainTextPassthrough(t *testing.T) {
	input := "Just plain text with no formatting"
	got := ToHTML(input)
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestListsPreserved(t *testing.T) {
	input := "- item 1\n- item 2"
	got := ToHTML(input)
	if !strings.Contains(got, "- item 1") {
		t.Errorf("dashes should be preserved, got: %q", got)
	}
}

func TestNestedFormatting(t *testing.T) {
	got := ToHTML("**bold and _italic_**")
	if !strings.Contains(got, "<b>") {
		t.Errorf("missing bold tag, got: %q", got)
	}
	if !strings.Contains(got, "<i>italic</i>") {
		t.Errorf("missing italic tag, got: %q", got)
	}
}

// --- Splitter Tests ---

func TestShortMessageNotSplit(t *testing.T) {
	text := strings.Repeat("a", 500)
	chunks := Split(text)
	if len(chunks) != 1 {
		t.Errorf("got %d chunks, want 1", len(chunks))
	}
}

func TestExact4096NotSplit(t *testing.T) {
	text := strings.Repeat("a", 4096)
	chunks := Split(text)
	if len(chunks) != 1 {
		t.Errorf("got %d chunks, want 1", len(chunks))
	}
}

func TestSplitAtParagraph(t *testing.T) {
	part1 := strings.Repeat("a", 3800)
	part2 := strings.Repeat("b", 1200)
	text := part1 + "\n\n" + part2
	chunks := Split(text)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0] != part1 {
		t.Errorf("first chunk length = %d, want 3800", len(chunks[0]))
	}
}

func TestSplitPreservesCodeBlock(t *testing.T) {
	before := strings.Repeat("x", 3900)
	code := "```go\n" + strings.Repeat("y", 300) + "\n```"
	text := before + "\n" + code
	chunks := Split(text)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// The code block should not be split across chunks.
	for _, chunk := range chunks {
		opens := strings.Count(chunk, "```")
		if opens%2 != 0 {
			t.Errorf("chunk has unmatched ``` fences: %q", chunk[:min(100, len(chunk))])
		}
	}
}

func TestMultipleSplits(t *testing.T) {
	text := strings.Repeat("a\n", 7500)
	chunks := Split(text)
	if len(chunks) < 4 {
		t.Errorf("got %d chunks, want >= 4", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > maxChunkSize {
			t.Errorf("chunk %d has %d chars, exceeds %d", i, len(c), maxChunkSize)
		}
	}
}

func TestNoBreakPoint(t *testing.T) {
	text := strings.Repeat("a", 5000)
	chunks := Split(text)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if len(chunks[0]) != 4096 {
		t.Errorf("first chunk = %d chars, want 4096", len(chunks[0]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
