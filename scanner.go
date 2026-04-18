package main

import (
	"encoding/json"
	"fmt"

	"github.com/playwright-community/playwright-go"
)

type formField struct {
	Label     string   `json:"label"`     // Human-readable question text
	FieldType string   `json:"fieldType"` // "text", "select", "radio", "textarea", "checkbox", "combobox"
	TagName   string   `json:"tagName"`   // kept for backward compat with Phase 3 UI
	Required  bool     `json:"required"`  // Whether the field is required
	Options   []string `json:"options"`   // Available options (for select/radio)
	Selector  string   `json:"selector"`  // Optional CSS selector hint (may be empty)
}

// ============================================================================
// VISUAL DOM SCANNER — Finds form fields by visible label text
// ============================================================================

func scanFormFields(page playwright.Page) []formField {
	jsScript := `() => {
		const fields = [];
		const seen = new Set();

		// Container-First Strategy
		const containerSelectors = [
			'div[role="group"]',
			'div.field-container',
			'div.input-container',
			'fieldset',
			'.form-group',
			'.field-wrapper',
		];

		const allContainers = new Set();

		// Find defined containers
		containerSelectors.forEach(sel => {
			document.querySelectorAll(sel).forEach(el => allContainers.add(el));
		});

		// Fallback: Find implicit containers (divs that have both a label and an input/select/textarea)
		document.querySelectorAll('div, li').forEach(el => {
			if (el.querySelector('label') && el.querySelector('input, select, textarea, [role="combobox"], [role="radio"]')) {
				allContainers.add(el);
			}
		});

		allContainers.forEach(container => {
			if (!container.offsetParent) return; // Must be visible

			// Find label
			let labelEl = container.querySelector('label');
			let labelText = '';
			let required = false;

			if (labelEl) {
				labelText = labelEl.innerText;
			} else {
				// Try to find legend or span acting as label
				const legendEl = container.querySelector('legend');
				if (legendEl) {
					labelText = legendEl.innerText;
				} else {
					// Search for something resembling a label
					const firstTextEl = container.querySelector('span, div');
					if (firstTextEl) {
						// Heuristics...
						if (firstTextEl.innerText && firstTextEl.innerText.length < 100) {
							labelText = firstTextEl.innerText;
						}
					}
				}
			}

			if (!labelText) return;

			labelText = labelText.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
			if (labelText.includes('*')) {
				required = true;
			}

			// Try checking if there's aria-required on any element within
			const reqEl = container.querySelector('[aria-required="true"], .required');
			if (reqEl) {
				required = true;
			}

			const cleanLabel = labelText.replace(/\s*\*\s*$/, '').replace(/\s*\*/, ' ').trim();
			if (!cleanLabel || cleanLabel.length > 100) return;

			if (seen.has(cleanLabel)) return;

			// Identify Input Type
			let fieldType = 'text';
			const inputEl = container.querySelector('input, select, textarea, [role="combobox"], [role="radio"], [role="listbox"]');
			let selector = '';

			if (inputEl) {
				// Try to compute a simple selector if it has an id or name
				if (inputEl.id) {
					selector = '#' + CSS.escape(inputEl.id);
				} else if (inputEl.name) {
					selector = '[name="' + CSS.escape(inputEl.name) + '"]';
				}

				const tag = inputEl.tagName.toLowerCase();
				if (tag === 'select') {
					fieldType = 'select';
				} else if (inputEl.getAttribute('role') === 'combobox' || inputEl.getAttribute('role') === 'listbox') {
					fieldType = 'combobox';
				} else if (inputEl.getAttribute('role') === 'radio') {
					fieldType = 'radio';
				} else if (tag === 'input') {
					const type = inputEl.getAttribute('type');
					if (type === 'radio') {
						fieldType = 'radio';
					} else if (type === 'checkbox') {
						fieldType = 'checkbox';
					} else {
						fieldType = 'text';
					}
				} else if (tag === 'textarea') {
					fieldType = 'text';
				}
			} else {
				return; // No input found in container
			}

			seen.add(cleanLabel);

			fields.push({
				label: cleanLabel,
				fieldType: fieldType,
				tagName: fieldType,
				required: required,
				options: [],
				selector: selector
			});
		});

		return fields;
	}`

	var fields []formField

	frames := append([]playwright.Frame{page.MainFrame()}, page.MainFrame().ChildFrames()...)

	for _, frame := range frames {
		frameResult, err := frame.Evaluate(jsScript, nil)
		if err == nil && frameResult != nil {
			var frameFields []formField
			jsonBytes, _ := json.Marshal(frameResult)
			json.Unmarshal(jsonBytes, &frameFields)
			if len(frameFields) > 0 {
				fields = append(fields, frameFields...)
			}
		}
	}

	if len(fields) == 0 {
		fmt.Printf("   ⚠️  Visual field scan found 0 fields.\n")
	}

	return fields
}
