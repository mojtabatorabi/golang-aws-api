package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/golang-aws-api/database"
)

// MockUser represents a user in our mock authentication system
type MockUser struct {
	ID          string
	Username    string
	Password    string
	Email       string
	Confirmed   bool
	AccessToken string
	CreatedAt   time.Time
}

// MockAuthProvider provides mock authentication functionality
type MockAuthProvider struct {
	mu sync.RWMutex
}

var (
	mockProvider = &MockAuthProvider{}
)

// GenerateToken generates a random token
func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// MockSignUp registers a new user in the mock system
func MockSignUp(ctx context.Context, username, password, email string) (*MockUser, error) {
	mockProvider.mu.Lock()
	defer mockProvider.mu.Unlock()

	// Check if user already exists
	existingUser, err := database.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}

	// Check if email already exists
	existingEmail, err := database.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if existingEmail != nil {
		return nil, errors.New("email already exists")
	}

	// Create new user in database
	dbUser, err := database.SaveUser(username, password, email)
	if err != nil {
		return nil, err
	}

	// Convert database user to mock user
	user := &MockUser{
		ID:        dbUser.ID,
		Username:  dbUser.Username,
		Password:  dbUser.Password,
		Email:     dbUser.Email,
		Confirmed: dbUser.Confirmed,
		CreatedAt: dbUser.CreatedAt,
	}

	return user, nil
}

// MockConfirmSignUp confirms a user's registration
func MockConfirmSignUp(ctx context.Context, username, code string) error {
	mockProvider.mu.Lock()
	defer mockProvider.mu.Unlock()

	// Check if user exists
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	// In a real system, we would verify the code
	// For mock purposes, we'll just confirm the user
	err = database.ConfirmUser(username)
	if err != nil {
		return err
	}

	return nil
}

// MockSignIn authenticates a user
func MockSignIn(ctx context.Context, username, password string) (*MockUser, error) {
	mockProvider.mu.RLock()
	defer mockProvider.mu.RUnlock()

	// Check if user exists
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Check if password matches
	if user.Password != password {
		return nil, errors.New("invalid password")
	}

	// Check if user is confirmed
	if !user.Confirmed {
		return nil, errors.New("user not confirmed")
	}

	// Generate access token
	accessToken := GenerateToken()

	// Convert database user to mock user
	mockUser := &MockUser{
		ID:          user.ID,
		Username:    user.Username,
		Password:    user.Password,
		Email:       user.Email,
		Confirmed:   user.Confirmed,
		AccessToken: accessToken,
		CreatedAt:   user.CreatedAt,
	}

	return mockUser, nil
}

// MockGetUser retrieves user information by access token
func MockGetUser(ctx context.Context, accessToken string) (*MockUser, error) {
	mockProvider.mu.RLock()
	defer mockProvider.mu.RUnlock()

	// In a real system, we would verify the token
	// For mock purposes, we'll just return a mock user
	return &MockUser{
		ID:          uuid.New().String(),
		Username:    "mockuser",
		AccessToken: accessToken,
		Confirmed:   true,
		CreatedAt:   time.Now(),
	}, nil
}

// MockSignOut signs out a user
func MockSignOut(ctx context.Context, accessToken string) error {
	// In a real system, we would invalidate the token
	// For mock purposes, we'll just return success
	return nil
}

// MockInit initializes the mock authentication system
func MockInit() {
	// Nothing to initialize
}

// MockAuthMiddleware provides a middleware that uses the mock authentication
func MockAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Check if the header has the Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		// Get the token
		token := parts[1]

		// Verify the token by getting user information
		_, err := MockGetUser(r.Context(), token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
