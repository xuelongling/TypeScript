package tsctests

import (
	"fmt"
	"strings"
	"testing"
)

func TestVersionSanitizationPreservesHelpHeaderWidth(t *testing.T) {
	t.Parallel()
	const columns = 120
	header := "tsc: The TypeScript Compiler - " + englishVersion
	output := outputSanitizer{lines: []string{fmt.Sprintf("%-120s", header)}}
	got := output.transformLines()
	if len(got) != columns {
		t.Fatalf("sanitized help header occupies %d columns; want %d", len(got), columns)
	}
	if !strings.HasPrefix(got, "tsc: The TypeScript Compiler - "+fakeEnglishVersion) {
		t.Fatalf("help header did not use the stable test version: %q", got)
	}
}

func TestVersionSanitizationPreservesColoredHelpHeaderWidth(t *testing.T) {
	t.Parallel()
	header := "tsc: The TypeScript Compiler - " + englishVersion
	output := outputSanitizer{lines: []string{fmt.Sprintf("%-115s", header) + "\x1b[44m     \x1b[39;49m"}}
	got := output.transformLines()
	visible := strings.NewReplacer("\x1b[44m", "", "\x1b[39;49m", "").Replace(got)
	if len(visible) != 120 {
		t.Fatalf("sanitized colored help header occupies %d columns; want 120", len(visible))
	}
	if !strings.HasSuffix(got, "\x1b[44m     \x1b[39;49m") {
		t.Fatalf("help icon style changed: %q", got)
	}
}
