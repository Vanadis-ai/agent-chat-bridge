package formatter

import (
	"strings"
)

const maxChunkSize = 4096

// Split divides text into chunks that fit within Telegram's message limit.
func Split(text string) []string {
	if len(text) <= maxChunkSize {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxChunkSize {
			chunks = append(chunks, remaining)
			break
		}

		chunk := remaining[:maxChunkSize]
		cutAt := findSplitPoint(chunk, remaining)
		chunks = append(chunks, remaining[:cutAt])
		remaining = strings.TrimLeft(remaining[cutAt:], "\n")
	}

	return chunks
}

func findSplitPoint(chunk, full string) int {
	// Try paragraph boundary.
	if idx := strings.LastIndex(chunk, "\n\n"); idx > 0 {
		if !isInsideCodeBlock(full[:idx]) {
			return idx
		}
	}

	// Try code block boundary: split before a code block that would be cut.
	if idx := findCodeBlockBoundary(chunk); idx > 0 {
		return idx
	}

	// Try line boundary.
	if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
		return idx
	}

	// Hard split at max size.
	return maxChunkSize
}

func isInsideCodeBlock(text string) bool {
	count := strings.Count(text, "```")
	return count%2 != 0
}

func findCodeBlockBoundary(chunk string) int {
	openCount := 0
	lines := strings.Split(chunk, "\n")
	pos := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if openCount > 0 {
				openCount--
			} else {
				openCount++
			}
		}
		pos += len(line) + 1
	}

	// If we are inside an open code block, find where it started.
	if openCount > 0 {
		lastOpen := strings.LastIndex(chunk, "```")
		beforeOpen := strings.LastIndex(chunk[:lastOpen], "\n")
		if beforeOpen > 0 {
			return beforeOpen
		}
	}
	return 0
}
