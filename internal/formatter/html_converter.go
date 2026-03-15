package formatter

import (
	"regexp"
	"strings"
)

var (
	fencedCodeRe = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)\\n?```")
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	boldRe       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe     = regexp.MustCompile(`(?:^|[^*])_(.+?)_`)
	linkRe       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// ToHTML converts a subset of Markdown to Telegram-compatible HTML.
func ToHTML(md string) string {
	// Extract code blocks first to protect them from other transforms.
	var codeBlocks []string
	placeholder := "\x00CODEBLOCK_%d\x00"

	result := fencedCodeRe.ReplaceAllStringFunc(md, func(match string) string {
		idx := len(codeBlocks)
		parts := fencedCodeRe.FindStringSubmatch(match)
		lang := parts[1]
		code := escapeHTML(parts[2])
		var block string
		if lang != "" {
			block = `<pre><code class="language-` + lang + `">` + code + "\n</code></pre>"
		} else {
			block = "<pre><code>" + code + "\n</code></pre>"
		}
		codeBlocks = append(codeBlocks, block)
		return sprintf(placeholder, idx)
	})

	// Extract inline code.
	result = inlineCodeRe.ReplaceAllStringFunc(result, func(match string) string {
		idx := len(codeBlocks)
		parts := inlineCodeRe.FindStringSubmatch(match)
		code := escapeHTML(parts[1])
		codeBlocks = append(codeBlocks, "<code>"+code+"</code>")
		return sprintf(placeholder, idx)
	})

	// Escape HTML in remaining text.
	result = escapeHTML(result)

	// Apply formatting.
	result = boldRe.ReplaceAllString(result, "<b>$1</b>")
	result = replaceItalic(result)
	result = linkRe.ReplaceAllString(result, `<a href="$2">$1</a>`)

	// Restore code blocks (they were already escaped).
	for i, block := range codeBlocks {
		result = strings.Replace(
			result, escapeHTML(sprintf(placeholder, i)), block, 1,
		)
	}

	return result
}

func replaceItalic(s string) string {
	return italicRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := italicRe.FindStringSubmatch(match)
		prefix := ""
		if len(match) > 0 && match[0] != '_' {
			prefix = string(match[0])
		}
		return prefix + "<i>" + parts[1] + "</i>"
	})
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func sprintf(format string, a int) string {
	return strings.Replace(format, "%d", itoa(a), 1)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}
