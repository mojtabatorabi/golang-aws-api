package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	// Get database connection details from environment variables
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "postgres")

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Count files
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to count files: %v", err)
	}

	fmt.Printf("Number of files in database: %d\n", count)

	// List file details
	rows, err := db.Query("SELECT id, name, s3_key, created_at FROM files ORDER BY created_at DESC")
	if err != nil {
		log.Fatalf("Failed to query files: %v", err)
	}
	defer rows.Close()

	fmt.Println("\nFile details:")
	fmt.Println("ID\t\tName\t\tS3 Key\t\tCreated At")
	fmt.Println("------------------------------------------------------------")
	for rows.Next() {
		var id, name, s3Key string
		var createdAt string
		if err := rows.Scan(&id, &name, &s3Key, &createdAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", id, name, s3Key, createdAt)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
