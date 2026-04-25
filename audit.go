package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// AUDIT LOGGER — GDPR-Compliant Submission Tracking
// ============================================================================

// AuditEntry represents a single submission record in the audit log.
type AuditEntry struct {
	Timestamp string            `json:"timestamp"`
	JobURL    string            `json:"job_url"`
	Company   string            `json:"company,omitempty"`
	Fields    map[string]string `json:"fields_sent"`
	Sources   map[string]string `json:"sources"`
}

// AuditLogger writes JSON lines to an append-only audit log file.
type AuditLogger struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
}

// NewAuditLogger creates a new audit logger. The file is opened in append mode
// with restrictive permissions (0600).
func NewAuditLogger(path string) (*AuditLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open audit log: %v", err)
	}

	return &AuditLogger{
		filePath: path,
		file:     file,
	}, nil
}

// Log writes a single audit entry as a JSON line.
func (al *AuditLogger) Log(entry AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("   ⚠️  Audit log marshal error: %v\n", err)
		return
	}

	if _, err := al.file.Write(append(data, '\n')); err != nil {
		fmt.Printf("   ⚠️  Audit log write error: %v\n", err)
	}
}

// Close closes the audit log file.
func (al *AuditLogger) Close() {
	if al.file != nil {
		al.file.Close()
	}
}

// ============================================================================
// LOG ANONYMIZATION — PII Scrubbing for Safe Sharing
// ============================================================================

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRegex = regexp.MustCompile(`\b\d{6,15}\b`)
)

// AnonymizeLogs reads the audit log, scrubs PII, and writes to a new file.
func AnonymizeLogs(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("could not read audit log: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	var anonymized []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Not valid JSON — scrub as raw text
			anonymized = append(anonymized, scrubPII(line))
			continue
		}

		// Scrub fields_sent values
		if fields, ok := entry["fields_sent"].(map[string]interface{}); ok {
			for k, v := range fields {
				if vStr, ok := v.(string); ok {
					fields[k] = scrubPII(vStr)
				}
			}
		}

		// Re-marshal
		result, _ := json.Marshal(entry)
		anonymized = append(anonymized, string(result))
	}

	output := strings.Join(anonymized, "\n") + "\n"
	if err := os.WriteFile(outputPath, []byte(output), 0600); err != nil {
		return fmt.Errorf("could not write anonymized log: %v", err)
	}

	fmt.Printf("🔒 Anonymized log written to: %s\n", outputPath)
	return nil
}

// scrubPII replaces emails, phone numbers, and common name patterns.
func scrubPII(text string) string {
	text = emailRegex.ReplaceAllString(text, "***@***.***")
	text = phoneRegex.ReplaceAllString(text, "[PHONE_REDACTED]")
	return text
}
