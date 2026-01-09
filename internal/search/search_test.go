package search

import (
	"strings"
	"testing"
)

func TestExtractRFC(t *testing.T) {
	// Test extracting section 3 from RFC 8259 (JSON specification)
	section, err := ExtractRFC(8259, "3")
	if err != nil {
		t.Fatalf("Failed to extract section: %v", err)
	}

	// Check that we got some content
	if section == "" {
		t.Error("Expected non-empty section content")
	}

	// Print the extracted section content for debugging
	t.Logf("Extracted section content:\n%s", section)

	// Check that the content contains expected text
	// Since we're falling back to text format, check for text-based content
	expectedPhrases := []string{
		"JSON",
		"value",
		"object",
		"array",
		"string",
		"number",
		"null",
		"false",
		"true",
	}

	// Print the section content for manual inspection
	t.Logf("Section content for manual inspection:\n%s", section)

	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(section), strings.ToLower(phrase)) {
			t.Errorf("Section content should contain '%s'", phrase)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return contains(s, substr) || contains(s, toLower(substr))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) != -1
}

func toLower(s string) string {
	// Simple toLower implementation for testing
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func indexOf(s, substr string) int {
	// Simple indexOf implementation for testing
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
