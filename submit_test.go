package main

import (
	"strings"
	"testing"
)

// ============================================================================
// SUBMIT LOGIC TESTS — Pure function tests (no Playwright required)
// ============================================================================

func TestFindInMemory(t *testing.T) {
	memory := map[string]string{
		"what is your salary expectation":          "50000 EUR",
		"are you legally authorized to work":       "Yes",
		"earliest start date":                      "01.07.2026",
		"how did you hear about this position":     "LinkedIn",
		"do you require visa sponsorship":          "No",
		"notice period":                            "3 months",
	}

	tests := []struct {
		name      string
		label     string
		wantFound bool
		wantValue string
	}{
		{
			name:      "Exact match (case-insensitive)",
			label:     "What is your salary expectation",
			wantFound: true,
			wantValue: "50000 EUR",
		},
		{
			name:      "Partial match — label contains key",
			label:     "Please tell us: what is your salary expectation?",
			wantFound: true,
			wantValue: "50000 EUR",
		},
		{
			name:      "Partial match — key contains label",
			label:     "salary expectation",
			wantFound: true,
			wantValue: "50000 EUR",
		},
		{
			name:      "No match",
			label:     "What is your favorite color?",
			wantFound: false,
			wantValue: "",
		},
		{
			name:      "Visa sponsorship match",
			label:     "Do you require visa sponsorship?",
			wantFound: true,
			wantValue: "No",
		},
		{
			name:      "Empty label matches first found (contains behavior)",
			label:     "",
			wantFound: true, // strings.Contains(key, "") is always true
		},
		{
			name:      "Extra whitespace",
			label:     "  Earliest Start Date  ",
			wantFound: true,
			wantValue: "01.07.2026",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, found := findInMemory(memory, tc.label)
			if found != tc.wantFound {
				t.Errorf("findInMemory(%q) found = %v, want %v", tc.label, found, tc.wantFound)
			}
			if found && tc.wantValue != "" && value != tc.wantValue {
				t.Errorf("findInMemory(%q) = %q, want %q", tc.label, value, tc.wantValue)
			}
		})
	}
}

func TestGetAnswerVariants(t *testing.T) {
	// Reset translations cache for test isolation
	translationsLoaded = false
	translationsCache = nil

	tests := []struct {
		name           string
		answer         string
		wantContains   string // A translated variant should contain this
		wantMinLen     int    // Minimum number of variants
	}{
		{
			name:         "German Yes",
			answer:       "Ja",
			wantContains: "Yes",
			wantMinLen:   2,
		},
		{
			name:         "English No",
			answer:       "No",
			wantContains: "Nein",
			wantMinLen:   2,
		},
		{
			name:         "Germany translation",
			answer:       "Deutschland",
			wantContains: "Germany",
			wantMinLen:   2,
		},
		{
			name:       "Unknown value — no translation",
			answer:     "42 bananas",
			wantMinLen: 1, // At least the original
		},
		{
			name:         "Case insensitive",
			answer:       "DEUTSCHLAND",
			wantContains: "Germany",
			wantMinLen:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			variants := getAnswerVariants(tc.answer)

			if len(variants) < tc.wantMinLen {
				t.Errorf("getAnswerVariants(%q) returned %d variants, want >= %d", tc.answer, len(variants), tc.wantMinLen)
			}

			// First variant should always be the original
			if variants[0] != tc.answer {
				t.Errorf("First variant should be the original: got %q, want %q", variants[0], tc.answer)
			}

			// Check that the expected translation is present
			if tc.wantContains != "" {
				found := false
				for _, v := range variants {
					if strings.Contains(v, tc.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("getAnswerVariants(%q) should contain %q, got %v", tc.answer, tc.wantContains, variants)
				}
			}
		})
	}
}

func TestCleanBoilerplate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNot string // These strings should NOT be in the cleaned output
		wantHas string // These strings SHOULD be in the cleaned output
	}{
		{
			name:    "Remove cookie notice",
			input:   "Software Engineer Position\nWe use cookies to improve your experience.\nJoin our dynamic team.",
			wantNot: "cookie",
			wantHas: "Software Engineer",
		},
		{
			name:    "Remove copyright",
			input:   "Full Stack Developer\n© 2025 Acme Corp. All rights reserved.\nResponsibilities include...",
			wantNot: "All rights reserved",
			wantHas: "Full Stack Developer",
		},
		{
			name:    "Remove privacy policy",
			input:   "Data Scientist Role\nPrivacy policy applies.\nML experience required.",
			wantNot: "Privacy policy",
			wantHas: "Data Scientist",
		},
		{
			name:    "Remove empty lines and short fragments",
			input:   "Senior Backend Engineer\n\n\na\n\nExciting opportunity in Berlin",
			wantHas: "Senior Backend Engineer",
		},
		{
			name:    "Truncate long text",
			input:   strings.Repeat("A very long job description line. ", 500),
			wantHas: "[... truncated for AI processing]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanBoilerplate(tc.input)

			if tc.wantNot != "" && strings.Contains(strings.ToLower(result), strings.ToLower(tc.wantNot)) {
				t.Errorf("cleanBoilerplate should remove '%s', but found it in result", tc.wantNot)
			}

			if tc.wantHas != "" && !strings.Contains(result, tc.wantHas) {
				t.Errorf("cleanBoilerplate should keep '%s', but it was removed", tc.wantHas)
			}
		})
	}
}

func TestLoadTranslations(t *testing.T) {
	// Reset cache
	translationsLoaded = false
	translationsCache = nil

	translations := loadTranslations()

	// Should have loaded at least some translations (if file exists)
	if len(translations) == 0 {
		t.Log("translations.json not found — this is expected in CI environments")
		return
	}

	// Check a known translation pair
	if val, ok := translations["münchen"]; ok {
		if val != "Munich" {
			t.Errorf("Expected münchen → Munich, got %q", val)
		}
	}

	if val, ok := translations["computer science"]; ok {
		if val != "Informatik" {
			t.Errorf("Expected computer science → Informatik, got %q", val)
		}
	}
}

func TestFindDateInProfile(t *testing.T) {
	profile := map[string]string{
		"date_of_birth": "1995-07-17",
		"start_date":    "2026-07-01",
		"full_name":     "Yehia Herbawi",
	}

	tests := []struct {
		name  string
		label string
		want  string
	}{
		{"Match date_of_birth", "date_of_birth", "1995-07-17"},
		{"Match start_date", "start_date", "2026-07-01"},
		{"No match", "phone_number", ""},
		// Note: "birthday" and "dob" don't directly match "date_of_birth" key
		// because strings.Contains("birthday", "date_of_birth") is false
		{"Birthday — no match (key mismatch)", "birthday", ""},
		{"Partial match via key containing date", "date_of_birth field", "1995-07-17"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findDateInProfile(profile, tc.label)
			if got != tc.want {
				t.Errorf("findDateInProfile(%q) = %q, want %q", tc.label, got, tc.want)
			}
		})
	}
}
