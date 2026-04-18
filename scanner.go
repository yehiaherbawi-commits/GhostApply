package main

import (
	"encoding/json"
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// ============================================================================
// VISUAL DOM SCANNER — Finds form fields by visible label text
// ============================================================================

// scanFormFields uses a visual-first JavaScript scanner that finds ALL form
// fields regardless of whether they use native HTML or custom ATS widgets.
// It uses a Brute Force Label Strategy to find any text acting as a label.
func scanFormFields(page playwright.Page) []formField {
	jsScript := `() => {
		const fields = [];
		const seen = new Set();

		// Brute Force Label Strategy: Scan for anything that looks like a label
		const allNodes = document.querySelectorAll('*');

		allNodes.forEach(el => {
			if (!el.offsetParent) return; // Must be visible

			const isLabelTag = el.tagName.toLowerCase() === 'label';
			const isAriaRequired = el.getAttribute('aria-required') === 'true';

			// We only want leaf-ish nodes or specific tags, avoid getting massive body texts
			if (!isLabelTag && !isAriaRequired && el.children.length > 2) return;

			let text = el.innerText;
			if (!text) return;
			// 1. TEXT CLEANING: Replace all newlines with spaces and trim
			text = text.replace(/\n/g, ' ').trim();

			// 2. LENGTH LIMIT: Ignore any text block longer than 60 characters
			if (text.length > 60 || text.length < 2) return;

			// 3. EXCLUSION LIST: Explicitly ignore specific text
			const lowerText = text.toLowerCase();
			const exclusions = ['register', 'please note', 'resume', 'cancel', 'next'];
			if (exclusions.some(ex => lowerText === ex || lowerText.startsWith(ex))) return;

			const hasAsterisk = text.includes('*');

			// If it's not a label tag, doesn't have an asterisk, and isn't aria-required, skip it
			// unless it explicitly has a label class
			const hasLabelClass = el.className && typeof el.className === 'string' && el.className.toLowerCase().includes('label');
			if (!isLabelTag && !hasAsterisk && !isAriaRequired && !hasLabelClass) return;

			// Skip if it's a button or link
			if (el.tagName.toLowerCase() === 'button' || el.tagName.toLowerCase() === 'a') return;

			// 4. PROXIMITY VALIDATION: Verify input element exists
			const parent = el.parentElement;
			if (!parent) return;

			const inputSelector = 'input, select, textarea, [role="combobox"]';
			let hasInput = false;

			// Check same parent container
			if (parent.querySelector(inputSelector)) {
				hasInput = true;
			} else {
				// Check immediately following in DOM (nextElementSibling of el or its parent)
				let next = el.nextElementSibling;
				if (next && (next.matches(inputSelector) || next.querySelector(inputSelector))) {
					hasInput = true;
				} else {
					next = parent.nextElementSibling;
					if (next && (next.matches(inputSelector) || next.querySelector(inputSelector))) {
						hasInput = true;
					}
				}
			}

			if (!hasInput) return;

			const cleanLabel = text.replace(/\s*\*\s*$/, '').replace(/\s*\*/, ' ').trim();
			if (seen.has(cleanLabel)) return;
			seen.add(cleanLabel);

			// Determine field type by looking at the next sibling or children
			// We just default to 'unknown' and let the Locator handle it,
			// but we can make a rough guess
			let fieldType = 'text'; // Default to text for brute force

			// We can leave Options empty since we rely on click-to-reveal now

			fields.push({
				label: cleanLabel,
				fieldType: fieldType,
				tagName: fieldType,
				required: hasAsterisk || isAriaRequired || el.classList?.contains('required'),
				options: [],
				selector: ''
			});
		});

		return fields;
	}`

	var fields []formField

	// Check MainFrame first
	result, err := page.Evaluate(jsScript, nil)
	if err == nil && result != nil {
		jsonBytes, _ := json.Marshal(result)
		json.Unmarshal(jsonBytes, &fields)
	}

	// IFRAME AWARENESS: SuccessFactors heavily utilizes iframes
	if len(fields) == 0 {
		fmt.Println("   ⚠️  0 fields found in MainFrame. Scanning ChildFrames (Iframe Fallback)...")
		for _, frame := range page.MainFrame().ChildFrames() {
			frameResult, err := frame.Evaluate(jsScript, nil)
			if err == nil && frameResult != nil {
				var frameFields []formField
				jsonBytes, _ := json.Marshal(frameResult)
				json.Unmarshal(jsonBytes, &frameFields)
				if len(frameFields) > 0 {
					fields = append(fields, frameFields...)
					// We could break here if we assume only one main iframe, but let's collect all
				}
			}
		}
	}

	if err != nil && len(fields) == 0 {
		fmt.Printf("   ⚠️  Visual field scan failed: %v\n", err)
	}

	return fields
}
