package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type Application struct {
	ID      int
	Date    string
	Company string
	Role    string
	Score   float64
	Status     string
	URL        string
	ReportPath string
}

func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS applications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT,
		company TEXT,
		role TEXT,
		score REAL,
		status TEXT,
		url TEXT,
		report_path TEXT
	);
	`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	return db
}

func SaveApplication(db *sql.DB, company string, role string, score float64, status string, jobURL string, reportPath string) error {
	insertQuery := `
	INSERT INTO applications (date, company, role, score, status, url, report_path) 
	VALUES (date('now'), ?, ?, ?, ?, ?, ?);
	`
	_, err := db.Exec(insertQuery, company, role, score, status, jobURL, reportPath)
	if err != nil {
		return fmt.Errorf("failed to insert application: %v", err)
	}

	app := Application{
		Company:    company,
		Role:       role,
		Score:      score,
		Status:     status,
		URL:        jobURL,
		ReportPath: reportPath,
	}
	UpdateMarkdownTracker(app)

	return nil
}

func GetApplications(db *sql.DB) ([]Application, error) {
	rows, err := db.Query("SELECT id, date, company, role, score, status, url, IFNULL(report_path, '') FROM applications ORDER BY score DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var app Application
		err := rows.Scan(&app.ID, &app.Date, &app.Company, &app.Role, &app.Score, &app.Status, &app.URL, &app.ReportPath)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}
