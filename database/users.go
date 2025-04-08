package database

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        string
	Username  string
	Password  string
	Email     string
	Confirmed bool
	CreatedAt time.Time
}

// SaveUser saves a new user to the database
func SaveUser(username, password, email string) (*User, error) {
	var user User
	userID := uuid.New().String()
	err := GetDB().QueryRow(`
		INSERT INTO users (id, username, password, email)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, password, email, confirmed, created_at
	`, userID, username, password, email).Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Confirmed, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func GetUserByUsername(username string) (*User, error) {
	var user User
	err := GetDB().QueryRow(`
		SELECT id, username, password, email, confirmed, created_at 
		FROM users 
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Confirmed, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(email string) (*User, error) {
	var user User
	err := GetDB().QueryRow(`
		SELECT id, username, password, email, confirmed, created_at 
		FROM users 
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Confirmed, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ConfirmUser confirms a user's email
func ConfirmUser(username string) error {
	_, err := GetDB().Exec(`
		UPDATE users 
		SET confirmed = true 
		WHERE username = $1
	`, username)
	return err
}
