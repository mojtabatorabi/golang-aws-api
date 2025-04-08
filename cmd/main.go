package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/yourusername/golang-aws-api/auth"
	"github.com/yourusername/golang-aws-api/database"
)

// Global variables
var (
	s3Client    *s3.Client
	sqsQueueURL string
	bucketName  string
)

// FileData represents the data structure for file uploads
type FileData struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ProcessingResult represents the result from Lambda processing
type ProcessingResult struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

func setupAWS() error {
	// Set up AWS configuration
	customResolver := aws.EndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if os.Getenv("ENV") == "local" {
			localstackHost := os.Getenv("LOCALSTACK_HOST")
			if localstackHost == "" {
				localstackHost = "localstack"
			}
			localstackPort := os.Getenv("LOCALSTACK_PORT")
			if localstackPort == "" {
				localstackPort = "4566"
			}
			return aws.Endpoint{
				URL:               fmt.Sprintf("http://%s:%s", localstackHost, localstackPort),
				SigningRegion:     "us-east-1",
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	}))

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
	)

	if os.Getenv("ENV") == "local" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
	}

	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)

	// Set bucket and queue names
	bucketName = os.Getenv("S3_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "my-test-bucket"
	}
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	if sqsQueueURL == "" {
		sqsQueueURL = "http://localhost:4566/000000000000/my-queue"
	}

	return nil
}

func main() {
	// Initialize AWS
	log.Println("Setting up AWS...")
	if err := setupAWS(); err != nil {
		log.Fatalf("Failed to setup AWS: %v", err)
	}
	log.Println("AWS setup completed")

	// Initialize database
	log.Println("Initializing database...")
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialization completed")

	// Initialize mock authentication
	log.Println("Initializing authentication...")
	auth.MockInit()
	log.Println("Authentication initialization completed")

	r := mux.NewRouter()

	// Public endpoints (no auth required)
	r.HandleFunc("/api/auth/signup", mockSignUpHandler).Methods("POST")
	r.HandleFunc("/api/auth/confirm", mockConfirmSignUpHandler).Methods("POST")
	r.HandleFunc("/api/auth/signin", mockSignInHandler).Methods("POST")
	r.HandleFunc("/api/files", uploadFileHandler).Methods("POST")

	// Protected endpoints (auth required)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(auth.MockAuthMiddleware)

	api.HandleFunc("/files/{id}", getFileHandler).Methods("GET")
	api.HandleFunc("/files/{id}/result", getResultHandler).Methods("GET")

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// MockSignUp handler
func mockSignUpHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := auth.MockSignUp(r.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		http.Error(w, "Failed to sign up: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User registered successfully. Please check your email for confirmation code.",
		"user_id": user.Username,
	})
}

// MockConfirmSignUp handler
func mockConfirmSignUpHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := auth.MockConfirmSignUp(r.Context(), req.Username, req.Code)
	if err != nil {
		http.Error(w, "Failed to confirm sign up: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Email confirmed successfully. You can now sign in.",
	})
}

// MockSignIn handler
func mockSignInHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := auth.MockSignIn(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, "Failed to sign in: "+err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": user.AccessToken,
		"id_token":     user.AccessToken, // For simplicity, we're using the same token
	})
}

// uploadFileHandler handles file uploads to S3
func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	var fileData FileData
	if err := json.NewDecoder(r.Body).Decode(&fileData); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate unique ID if not provided
	if fileData.ID == "" {
		fileData.ID = uuid.New().String()
	}

	fileData.CreatedAt = time.Now()

	// Save file metadata to database
	s3Key := fmt.Sprintf("files/%s/%s", fileData.ID, fileData.Name)
	log.Printf("Saving file metadata to database: id=%s, name=%s, s3_key=%s", fileData.ID, fileData.Name, s3Key)
	_, err := database.GetDB().Exec(
		"INSERT INTO files (id, name, s3_key, created_at) VALUES ($1, $2, $3, $4)",
		fileData.ID, fileData.Name, s3Key, fileData.CreatedAt,
	)
	if err != nil {
		log.Printf("Error saving to database: %v", err)
		http.Error(w, "Error saving file metadata", http.StatusInternalServerError)
		return
	}

	// Upload content to S3
	log.Printf("Uploading to S3: bucket=%s, key=%s", bucketName, s3Key)
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
		Body:   strings.NewReader(fileData.Content),
	})
	if err != nil {
		log.Printf("Error uploading to S3: %v", err)
		http.Error(w, "Error uploading file", http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully uploaded to S3")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":      fileData.ID,
		"status":  "uploaded",
		"message": "File uploaded successfully and processing started",
	})
}

// getFileHandler retrieves file information
func getFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	var fileData FileData
	var s3Key string

	err := database.GetDB().QueryRow(
		"SELECT id, name, s3_key, created_at FROM files WHERE id = $1",
		fileID,
	).Scan(&fileData.ID, &fileData.Name, &s3Key, &fileData.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			log.Printf("Database query error: %v", err)
			http.Error(w, "Error retrieving file", http.StatusInternalServerError)
		}
		return
	}

	// Get file content from S3
	result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		log.Printf("Error retrieving from S3: %v", err)
		http.Error(w, "Error retrieving file content", http.StatusInternalServerError)
		return
	}
	defer result.Body.Close()

	// Read content
	content, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("Error reading S3 content: %v", err)
		http.Error(w, "Error reading file content", http.StatusInternalServerError)
		return
	}
	fileData.Content = string(content)

	// Return file data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileData)
}

// getResultHandler retrieves processing results
func getResultHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	var result ProcessingResult

	err := database.GetDB().QueryRow(
		"SELECT id, status, result, created_at FROM processing_results WHERE file_id = $1",
		fileID,
	).Scan(&result.ID, &result.Status, &result.Result, &result.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Check if file exists first
			var exists bool
			err = database.GetDB().QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE id = $1)", fileID).Scan(&exists)
			if err != nil || !exists {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}

			// File exists but processing not complete
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "processing",
				"message": "Processing not complete or not started",
			})
			return
		} else {
			log.Printf("Database query error: %v", err)
			http.Error(w, "Error retrieving processing result", http.StatusInternalServerError)
			return
		}
	}

	// Return processing result
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
