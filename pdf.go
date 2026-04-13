package main

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// GeneratePDF takes the tailored content and saves it as a PDF file
func GeneratePDF(pw *playwright.Playwright, content *CVContent, filename string) error {
	browser, _ := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	defer browser.Close()

	page, _ := browser.NewPage()

	// Minimalist, ATS-friendly HTML template
	htmlTemplate := fmt.Sprintf(`
	<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; padding: 50px; }
			h1 { color: #000; border-bottom: 2px solid #000; }
			h2 { color: #444; margin-top: 20px; }
			p { font-size: 12pt; }
			ul { padding-left: 20px; }
			li { margin-bottom: 10px; font-size: 12pt; }
		</style>
	</head>
	<body>
		<h1>%s</h1>
		<p>%s | %s</p>
		<h2>Professional Summary</h2>
		<p>%s</p>
		<h2>Key Experience & Projects</h2>
		<ul>`, content.Name, content.Location, content.Role, content.Summary)

	for _, bullet := range content.Experience {
		htmlTemplate += fmt.Sprintf("<li>%s</li>", bullet)
	}

	htmlTemplate += "</ul></body></html>"

	// Set the content of the page
	page.SetContent(htmlTemplate)

	// Print to PDF
	_, err := page.PDF(playwright.PagePdfOptions{
		Path:            playwright.String(filename),
		Format:          playwright.String("A4"),
		PrintBackground: playwright.Bool(true),
	})

	if err != nil {
		return fmt.Errorf("failed to create PDF: %v", err)
	}

	fmt.Printf("📄 PDF Resume generated: %s\n", filename)
	return nil
}
