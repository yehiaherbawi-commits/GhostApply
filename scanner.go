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
func scanFormFields(page playwright.Page) []formField {
	jsScript := `() => {
		const fields = [];
		const seen = new Set();

		// Collect ALL potential label/question elements
		const allLabels = document.querySelectorAll(
			'label, legend, ' +
			'[class*="label" i]:not(input):not(button):not(select):not(textarea), ' +
			'[id*="label" i]:not(input):not(button):not(select):not(textarea)'
		);

		allLabels.forEach(el => {
			if (!el.offsetParent) return;
			const text = el.innerText?.trim();
			if (!text || text.length > 200 || text.length < 2) return;
			if (seen.has(text)) return;
			// Skip if element contains interactive children (it's a wrapper)
			if (el.querySelector('input:not([type="hidden"]), select, textarea, [role="combobox"]')) return;

			seen.add(text);

			// Walk up to the form group container
			const container = el.closest(
				'.form-group, .field-container, fieldset, ' +
				'[class*="field" i], [class*="row" i], [class*="form" i], ' +
				'div, li, section, td'
			) || el.parentElement;
			if (!container) return;

			let fieldType = 'unknown';
			let options = [];
			const isRequired = text.includes('*') ||
				el.classList?.contains('required') ||
				container.querySelector('[aria-required="true"]') !== null;

			// Text input
			const textInput = container.querySelector(
				'input[type="text"], input[type="email"], input[type="tel"], ' +
				'input[type="url"], input[type="number"], input:not([type])'
			);
			if (textInput && textInput.offsetParent && textInput.type !== 'hidden') {
				fieldType = 'text';
			}

			// Textarea
			if (container.querySelector('textarea')?.offsetParent) {
				fieldType = 'textarea';
			}

			// Native select
			const sel = container.querySelector('select');
			if (sel && sel.offsetParent) {
				fieldType = 'dropdown';
				options = Array.from(sel.options).map(o => o.text.trim()).filter(
					t => t && t !== 'Please select' && t !== '--' && t !== '' && t !== 'Select...'
				);
			}

			// Custom dropdown (role=combobox etc.)
			const combo = container.querySelector(
				'[role="combobox"], [role="listbox"], [aria-haspopup="listbox"], ' +
				'[class*="dropdown" i]:not(label), [class*="combobox" i]'
			);
			if (combo && combo.offsetParent) {
				fieldType = 'dropdown';
			}

			// Native radio buttons
			const radios = container.querySelectorAll('input[type="radio"]');
			if (radios.length > 0) {
				fieldType = 'radio';
				radios.forEach(r => {
					let ol = '';
					if (r.id) {
						const lbl = document.querySelector('label[for="' + r.id + '"]');
						if (lbl) ol = lbl.innerText.trim();
					}
					if (!ol) {
						const p = r.closest('label');
						if (p) ol = p.innerText.trim();
					}
					if (ol) options.push(ol);
				});
			}

			// Custom radio/toggle
			const cr = container.querySelectorAll(
				'[role="radio"], [class*="toggle" i], [class*="choice" i]'
			);
			if (cr.length > 0 && fieldType !== 'radio') {
				fieldType = 'radio';
				cr.forEach(r => {
					const t = r.innerText?.trim();
					if (t && t.length < 50) options.push(t);
				});
			}

			if (fieldType === 'unknown') return;

			const cleanLabel = text.replace(/\\s*\\*\\s*$/, '').replace(/\\s*\\*/, ' ').trim();

			fields.push({
				label: cleanLabel,
				fieldType: fieldType,
				tagName: fieldType,
				required: isRequired,
				options: [...new Set(options)],
				selector: ''
			});
		});

		return fields;
	}`

	result, err := page.Evaluate(jsScript, nil)
	if err != nil {
		fmt.Printf("   ⚠️  Visual field scan failed: %v\n", err)
		return nil
	}

	var fields []formField
	jsonBytes, _ := json.Marshal(result)
	json.Unmarshal(jsonBytes, &fields)

	return fields
}
