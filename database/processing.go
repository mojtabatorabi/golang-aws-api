package database

import (
	"database/sql"
	"time"
)

type ProcessingResult struct {
	ID        string
	FileID    string
	Status    string
	Result    string
	CreatedAt time.Time
}

// SaveProcessingResult saves a new processing result to the database
func SaveProcessingResult(fileID, status, result string) error {
	_, err := GetDB().Exec(`
		INSERT INTO processing_results (file_id, status, result)
		VALUES ($1, $2, $3)
	`, fileID, status, result)
	return err
}

// GetProcessingResultByFileID retrieves the processing result for a specific file
func GetProcessingResultByFileID(fileID string) (*ProcessingResult, error) {
	var pr ProcessingResult
	err := GetDB().QueryRow(`
		SELECT id, file_id, status, result, created_at 
		FROM processing_results 
		WHERE file_id = $1
		ORDER BY created_at DESC 
		LIMIT 1
	`, fileID).Scan(&pr.ID, &pr.FileID, &pr.Status, &pr.Result, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

// UpdateProcessingResult updates the status and result of a processing result
func UpdateProcessingResult(fileID, status, result string) error {
	_, err := GetDB().Exec(`
		UPDATE processing_results 
		SET status = $1, result = $2 
		WHERE file_id = $3
	`, status, result, fileID)
	return err
}
