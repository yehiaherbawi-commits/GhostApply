package main

import (
	"os"
	"strings"
	"testing"
)

// ============================================================================
// AUDIT LOGGER TESTS — GDPR Compliance Verification
// ============================================================================

func TestAuditLogger_WriteAndRead(t *testing.T) {
	tmpFile := "test_audit.log"
	defer os.Remove(tmpFile)

	logger, err := NewAuditLogger(tmpFile)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	// Write entries
	logger.Log(AuditEntry{
		JobURL:  "https://example.com/job/123",
		Company: "TestCorp",
		Fields:  map[string]string{"Email": "user@test.com", "Phone": "0157123456"},
		Sources: map[string]string{"Email": "cv", "Phone": "profile"},
	})

	logger.Log(AuditEntry{
		JobURL:  "https://example.com/job/456",
		Company: "AcmeInc",
		Fields:  map[string]string{"Name": "John Doe"},
		Sources: map[string]string{"Name": "cv"},
	})

	logger.Close()

	// Read and validate
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Could not read audit log: %v", err)
	}

	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	if len(lines) != 2 {
		t.Errorf("Expected 2 log lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "TestCorp") {
		t.Errorf("First entry should contain 'TestCorp'")
	}
	if !strings.Contains(lines[0], "user@test.com") {
		t.Errorf("First entry should contain the full email (Option A)")
	}
	if !strings.Contains(lines[1], "AcmeInc") {
		t.Errorf("Second entry should contain 'AcmeInc'")
	}
}

func TestAnonymizeLogs(t *testing.T) {
	inputFile := "test_audit_anon_input.log"
	outputFile := "test_audit_anon_output.log"
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)

	// Write a test log with PII
	content := `{"timestamp":"2026-04-26T00:00:00Z","job_url":"https://example.com","company":"TestCorp","fields_sent":{"Email":"user@test.com","Phone":"015712345678","Name":"John Doe"},"sources":{"Email":"cv","Phone":"profile","Name":"cv"}}
`
	os.WriteFile(inputFile, []byte(content), 0600)

	// Anonymize
	err := AnonymizeLogs(inputFile, outputFile)
	if err != nil {
		t.Fatalf("AnonymizeLogs() error: %v", err)
	}

	// Read anonymized output
	anonData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Could not read anonymized log: %v", err)
	}
	anonContent := string(anonData)

	// Email should be scrubbed
	if strings.Contains(anonContent, "user@test.com") {
		t.Error("Anonymized log should not contain original email")
	}
	if !strings.Contains(anonContent, "***@***.***") {
		t.Error("Anonymized log should contain scrubbed email pattern")
	}

	// Phone should be scrubbed
	if strings.Contains(anonContent, "015712345678") {
		t.Error("Anonymized log should not contain original phone number")
	}

	// Company name should be preserved (it's not PII)
	if !strings.Contains(anonContent, "TestCorp") {
		t.Error("Anonymized log should preserve company name")
	}
}

func TestScrubPII(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNot string
	}{
		{"Email", "Contact me at john@example.com please", "john@example.com"},
		{"Phone long", "Call 015712345678 for info", "015712345678"},
		{"Multiple emails", "a@b.com and c@d.org", "a@b.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scrubPII(tc.input)
			if strings.Contains(result, tc.wantNot) {
				t.Errorf("scrubPII should remove '%s' from output, got: %s", tc.wantNot, result)
			}
		})
	}
}
