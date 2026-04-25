# 👻 GhostApply

**GhostApply** is an advanced, AI-powered job application automation agent built in Go. It leverages Google's Gemini AI and Playwright browser automation to scrape job descriptions, evaluate compatibility, tailor resumes in real-time, and auto-fill application forms — all while staying stealthy and secure.

---

## ✨ Key Features

### 🧠 AI-Powered Pipeline
- **Intelligent Scraping**: Extracts job descriptions using 30+ cascading CSS selectors, recursive iframe search, and boilerplate cleanup.
- **AI Evaluation**: Analyzes JD against your CV via Gemini and returns a compatibility score (1-10).
- **AI Writer & Critic Loop**: Drafts a tailored CV, then a Critic agent reviews it. Up to 3 revision cycles with **best-draft tracking** (highest Critic score always wins).
- **Application Strategist**: Drafts "Why this company?" and "Technical challenge" answers with interactive review before submission.
- **Multi-CV Selection**: If a `cvs/` directory exists, Gemini AI picks the best-matching CV for each job automatically.

### 🤖 Automation & Form-Filling
- **Auto-Fill Engine**: Navigates ATS forms and populates fields using CV data, agent profile, and self-learning QA memory.
- **AI Selector Validation**: Validates every AI-returned CSS selector via `Locator.Count()` before interacting — falls back to visual scanning on failure.
- **Unknown Field Prompting**: Collects unrecognized required fields into a batch and prompts the user for answers (saved to memory for next time).
- **Date Picker Handler**: Supports native `input[type="date"]` and custom date widget libraries.
- **Tag Input Handler**: Handles select2, tagify, and generic chip/tag inputs.
- **Smart Translation**: 170 DE↔EN translation pairs loaded from `translations.json` for international ATS portals.

### 🔒 Security & Stealth
- **AES-256-GCM Encryption**: Credentials and QA memory are encrypted at rest (`.enc` files). Auto-migrates plaintext on first run.
- **Stealth Browser**: Hides `navigator.webdriver`, spoofs plugins/languages, uses realistic viewport (1920×1080) and User-Agent.
- **Human-Like Delays**: All pauses use a truncated normal distribution (`humanDelay`) instead of fixed `time.Sleep`.
- **CAPTCHA Detection**: Detects reCAPTCHA, hCaptcha, and generic CAPTCHAs with user-prompted manual solve (120s timeout).
- **MFA Detection**: Detects OTP/2FA prompts via input fields and text signals (EN + DE) with 180s manual solve timeout.

### 📊 Cost Control & Compliance
- **Token Budget**: Global `TokenBudget` tracker (default 1M tokens) with mutex-safe counting across all workers.
- **GDPR Audit Logger**: JSON Lines audit log (`audit.log`) recording every field submitted, with source tracking (cv/profile/memory/manual/ai).
- **PII Anonymization**: `--anonymize-logs` command scrubs emails and phone numbers for safe log sharing.
- **Dry-Run Mode**: `--dry-run` outputs a full JSON report without opening a browser or filling any forms.

### 📊 Dashboard & Batch Processing
- **Interactive TUI**: Bubble Tea dashboard for tracking application history, scores, and statuses.
- **Batch Processing**: Process multiple job URLs concurrently from `targets.txt` with per-worker budget checks.

---

## 🛠️ Tech Stack

| Technology | Purpose |
|------------|---------|
| **Go** | Core application logic |
| **Gemini AI** | LLM for evaluation, drafting, reviewing, CV selection |
| **Playwright** | Browser automation for scraping and form-filling |
| **SQLite** | Local database for application tracking |
| **Bubble Tea** | TUI (Terminal User Interface) framework |
| **AES-256-GCM** | Encryption for credentials and sensitive data |

---

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) (v1.26+)
- [Playwright for Go](https://github.com/playwright-community/playwright-go) dependencies
- A Google Cloud Project with the **Generative AI API** enabled and an API Key

### Installation

```bash
git clone https://github.com/yehiaherbawi-commits/GhostApply.git
cd GhostApply
go mod tidy
```

Install Playwright browsers:
```bash
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
```

### Environment Setup

Create a `.env` file:
```env
GEMINI_API_KEY=your_api_key_here
```

### Configuration Files

| File | Purpose |
|------|---------|
| `my_cv.txt` | Your base resume in plain text (fallback if `cvs/` doesn't exist) |
| `cvs/` | Directory of multiple CV `.txt` files (AI picks best match per job) |
| `agent_profile.json` | Personal details and preferences for form-filling |
| `targets.txt` | List of job URLs for batch mode |
| `translations.json` | DE↔EN translation pairs (auto-loaded, 170 included) |

---

## 📖 Usage

### Single Job Application
```bash
go run . "https://company.com/job-listing-url"
```

### Dry-Run Mode (No Form Filling)
```bash
go run . --dry-run "https://company.com/job-listing-url"
```
Outputs a JSON report (`dry_run_<Company>.json`) with tailored CV, draft answers, and token usage.

### Batch Mode
```bash
go run . --batch
```

### Anonymize Audit Logs
```bash
go run . --anonymize-logs
```

### View Dashboard
```bash
go run .
```

---

## 🧪 Testing

```bash
go test -v ./...
```

**25 tests** across 4 test files:

| Test File | Coverage |
|-----------|----------|
| `crypto_test.go` | AES encryption/decryption, key management, migration |
| `ai_test.go` | JSON cleaning, CV tailoring, API failure handling |
| `submit_test.go` | Memory lookup, translations, boilerplate cleaning, date matching |
| `audit_test.go` | Audit logging, PII anonymization, scrubbing |

---

## 📂 Project Structure

```
GhostApply/
├── main.go          # Entry point, CLI flags, orchestration
├── ai.go            # Gemini AI integration, TokenBudget tracker
├── auth.go          # Encrypted credential vault, CAPTCHA/MFA detection
├── submit.go        # Auto-fill engine, QA memory, selector validation
├── scanner.go       # Visual DOM scanner for form fields
├── scraper.go       # Smart JD extraction with cascading selectors
├── stealth.go       # Anti-bot: stealth browser, humanDelay, humanType
├── crypto.go        # AES-256-GCM encryption utilities
├── audit.go         # GDPR audit logger with PII anonymization
├── multicv.go       # AI-driven multi-CV selection
├── batch.go         # Concurrent batch processing with budget checks
├── pdf.go           # PDF generation from tailored CVs
├── ui.go            # Bubble Tea TUI dashboard
├── db.go            # SQLite database operations
├── translations.json # 170 DE↔EN translation pairs
├── *_test.go        # Test suites (25 tests)
├── .gitignore       # Comprehensive exclusion rules
└── .env             # API key (not committed)
```

### Security Files (Auto-Generated, Never Committed)

| File | Purpose |
|------|---------|
| `.jobagent.key` | AES-256 encryption key (auto-generated on first run) |
| `credentials.enc` | Encrypted login credentials |
| `qa_memory.enc` | Encrypted QA memory bank |
| `audit.log` | Submission audit trail |

---

## ⚠️ Security Notes

- **First Run**: `credentials.json` and `qa_memory.json` are automatically encrypted to `.enc` files. Plaintext originals are deleted.
- **Git History**: If plaintext credentials were previously committed, run `git filter-branch` or [BFG Repo-Cleaner](https://rtyley.github.io/bfg-repo-cleaner/) to scrub history.
- **Encryption Key**: `.jobagent.key` is stored locally with restrictive permissions. Never share or commit this file.

---

## 🛡️ License

Distributed under the MIT License. See `LICENSE` for more information.

---

*Built with ❤️ — Powered by Gemini AI & Playwright*
