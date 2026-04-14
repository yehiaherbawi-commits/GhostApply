# 🚀 JobAgent

**JobAgent** is an advanced, AI-powered automation tool designed to streamline the job application process. It leverages Google's Gemini AI and Playwright browser automation to scrape job descriptions, evaluate compatibility, tailor resumes in real-time, and even auto-fill application forms.

---

## ✨ Key Features

- **🔍 Intelligent Scraping**: Automatically extracts job details and descriptions from any URL using Playwright.
- **🧠 AI Evaluation**: Uses Gemini AI to analyze job descriptions against your CV (`my_cv.txt`) and provide a compatibility score.
- **✍️ AI Writer & Critic**: 
    - **Writer**: Drafts a CV tailored specifically to the job.
    - **Critic**: Reviews the draft against the job description for quality assurance, providing feedback for up to 3 revision cycles.
- **📄 PDF Generation**: Converts tailored CVs into professional PDF resumes on-the-fly.
- **📝 Application Strategist**: Drafts custom answers for common application questions (e.g., "Why do you want to work here?").
- **🤖 Auto-Fill (The Final Boss)**: Automatically navigates to application forms and populates fields using your tailored data.
- **📊 Interactive Dashboard**: A terminal-based UI (Bubble Tea) to track application history, scores, and statuses.
- **⚡ Batch Processing**: Process multiple job listings at once using a target list.

---

## 🛠️ Tech Stack

- **Go**: Core application logic.
- **Gemini AI**: Large Language Model for evaluation, drafting, and reviewing.
- **Playwright**: Browser automation for scraping and form-filling.
- **SQLite**: Local database for application tracking.
- **Bubble Tea**: TUI (Terminal User Interface) framework for the dashboard.

---

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) (v1.26 or later recommended)
- [Playwright for Go](https://github.com/playwright-community/playwright-go) dependencies.
- A Google Cloud Project with the **Generative AI API** enabled and an API Key.

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/JobAgent.git
   cd JobAgent
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Setup environment variables:
   Create a `.env` file in the root directory and add your Gemini API key:
   ```env
   GEMINI_API_KEY=your_api_key_here
   ```

4. Install Playwright browsers:
   ```bash
   go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
   ```

---

## 📖 Usage

### 🧪 Configuration
Ensure the following files are set up in the root directory:
- `my_cv.txt`: Your base resume in plain text.
- `agent_profile.json`: Your personal details and preferences.
- `targets.txt`: A list of job URLs (for batch mode).

### 🏃 Running the Agent

**Single Job Application:**
```bash
go run . "https://company.com/job-listing-url"
```

**Batch Mode:**
```bash
go run . --batch
```

**View Dashboard:**
```bash
go run .
```

---

## 📂 Project Structure

- `main.go`: Entry point and application orchestration.
- `ai.go`: Gemini AI integration for evaluation and drafting.
- `submit.go`: Playwright logic for automated form submission.
- `scraper.go`: Web scraping logic.
- `pdf.go`: PDF generation from tailored CVs.
- `ui.go`: Bubble Tea TUI components.
- `db.go`: SQLite database operations.
- `batch.go`: Logic for processing multiple targets.

---

## 🛡️ License

Distributed under the MIT License. See `LICENSE` for more information.

---

*Generated with ❤️ by Antigravity AI*
