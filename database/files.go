package database

import (
	"database/sql"
	"time"
)

type File struct {
	ID        string
	Name      string
	S3Key     string
	CreatedAt time.Time
}

// GetAllFiles retrieves all files from the database
func GetAllFiles() ([]File, error) {
	rows, err := GetDB().Query(`
		SELECT id, name, s3_key, created_at 
		FROM files 
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Name, &f.S3Key, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// SaveFile saves a new file to the database
func SaveFile(name, s3Key string) (*File, error) {
	var f File
	err := GetDB().QueryRow(`
		INSERT INTO files (name, s3_key)
		VALUES ($1, $2)
		RETURNING id, name, s3_key, created_at
	`, name, s3Key).Scan(&f.ID, &f.Name, &f.S3Key, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetFileByID retrieves a file by its ID
func GetFileByID(id string) (*File, error) {
	var f File
	err := GetDB().QueryRow(`
		SELECT id, name, s3_key, created_at 
		FROM files 
		WHERE id = $1
	`, id).Scan(&f.ID, &f.Name, &f.S3Key, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}
