package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// InitDB initializes the database connection and creates necessary tables
func InitDB() error {
	// Set up PostgreSQL connection
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	log.Printf("Attempting to connect to database at %s:%s...", dbHost, dbPort)

	// Retry connection with backoff
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		log.Printf("Connection attempt %d of %d", i+1, maxRetries)
		db, err = sql.Open("postgres", dbInfo)
		if err != nil {
			log.Printf("Failed to connect to database (attempt %d): %v", i+1, err)
			if i < maxRetries-1 {
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			return fmt.Errorf("failed to connect to database after %d attempts: %v", maxRetries, err)
		}

		// Test database connection
		err = db.Ping()
		if err == nil {
			log.Printf("Successfully connected to database")
			break
		}
		log.Printf("Failed to ping database (attempt %d): %v", i+1, err)
		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		return fmt.Errorf("failed to ping database after %d attempts: %v", maxRetries, err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("Creating database tables...")
	// Create tables if not exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			confirmed BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			s3_key TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		
		CREATE TABLE IF NOT EXISTS processing_results (
			id TEXT PRIMARY KEY,
			file_id TEXT NOT NULL REFERENCES files(id),
			status TEXT NOT NULL,
			result TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}
	log.Printf("Database tables created successfully")

	return nil
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	if db == nil {
		log.Fatal("Database connection not initialized. Make sure InitDB() is called before using the database.")
	}
	return db
}
