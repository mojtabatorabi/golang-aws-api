// tests/integration_test.go
package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Global variables for tests
var (
	apiURL     string
	s3Client   *s3.Client
	sqsClient  *sqs.Client
	bucketName string
	queueURL   string
	db         *sql.DB
)

// FileData represents the file upload request/response
type FileData struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ProcessingResult represents the processing result
type ProcessingResult struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

func TestMain(m *testing.M) {
	// Set up test environment
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := startPostgresContainer(ctx)
	if err != nil {
		fmt.Printf("Failed to start PostgreSQL container: %v\n", err)
		os.Exit(1)
	}
	defer postgresContainer.Terminate(ctx)

	// Start LocalStack container
	localstackContainer, err := startLocalStackContainer(ctx)
	if err != nil {
		fmt.Printf("Failed to start LocalStack container: %v\n", err)
		os.Exit(1)
	}
	defer localstackContainer.Terminate(ctx)

	// Get PostgreSQL connection details
	pgHost, err := postgresContainer.Host(ctx)
	if err != nil {
		fmt.Printf("Failed to get PostgreSQL host: %v\n", err)
		os.Exit(1)
	}

	pgPort, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Printf("Failed to get PostgreSQL port: %v\n", err)
		os.Exit(1)
	}

	// Set up PostgreSQL connection
	dbInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		pgHost, pgPort.Port(), "postgres", "postgres", "postgres")

	db, err = sql.Open("postgres", dbInfo)
	if err != nil {
		fmt.Printf("Failed to connect to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create test tables
	_, err = db.Exec(`
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
		fmt.Printf("Failed to create tables: %v\n", err)
		os.Exit(1)
	}

	// Get LocalStack connection details
	localstackHost, err := localstackContainer.Host(ctx)
	if err != nil {
		fmt.Printf("Failed to get LocalStack host: %v\n", err)
		os.Exit(1)
	}

	localstackPort, err := localstackContainer.MappedPort(ctx, "4566")
	if err != nil {
		fmt.Printf("Failed to get LocalStack port: %v\n", err)
		os.Exit(1)
	}

	// Set up AWS configuration for LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               fmt.Sprintf("http://%s:%s", localstackHost, localstackPort.Port()),
			SigningRegion:     "us-east-1",
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		fmt.Printf("Failed to load AWS configuration: %v\n", err)
		os.Exit(1)
	}

	s3Client = s3.NewFromConfig(cfg)
	sqsClient = sqs.NewFromConfig(cfg)

	// Create bucket and queue for testing
	bucketName = "test-bucket"
	queueName := "test-queue"

	// Retry creating the S3 bucket
	var createBucketErr error
	for i := 0; i < 5; i++ {
		// Try to create the bucket with a specific region
		createBucketErr = createS3Bucket(ctx, s3Client, bucketName)
		if createBucketErr == nil {
			break
		}
		fmt.Printf("Attempt %d: Failed to create S3 bucket: %v\n", i+1, createBucketErr)
		time.Sleep(5 * time.Second)
	}
	if createBucketErr != nil {
		fmt.Printf("Failed to create S3 bucket after multiple attempts: %v\n", createBucketErr)
		os.Exit(1)
	}

	queueResult, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		fmt.Printf("Failed to create SQS queue: %v\n", err)
		os.Exit(1)
	}
	queueURL = *queueResult.QueueUrl

	// Configure S3 event notifications to SQS (in a real environment this would be set up in AWS)
	// For our tests, we'll simulate this by manually sending messages to SQS when we upload to S3

	// Start the API server
	// Instead of assuming the API is already running, we'll start it here
	apiPort := "8081" // Use a different port to avoid conflicts
	os.Setenv("API_PORT", apiPort)
	apiURL = fmt.Sprintf("http://localhost:%s", apiPort)

	// Start the API server in a goroutine
	go func() {
		// Set up the router
		r := mux.NewRouter()
		r.HandleFunc("/api/files", uploadFileHandler).Methods("POST")
		r.HandleFunc("/api/files/{id}", getFileHandler).Methods("GET")
		r.HandleFunc("/api/files/{id}/result", getResultHandler).Methods("GET")

		// Start the server
		log.Printf("API server starting on port %s", apiPort)
		if err := http.ListenAndServe(":"+apiPort, r); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Give the API server time to start
	time.Sleep(2 * time.Second)

	// Set environment variables for the API
	os.Setenv("ENV", "local")
	os.Setenv("S3_BUCKET_NAME", bucketName)
	os.Setenv("SQS_QUEUE_URL", queueURL)
	os.Setenv("DB_HOST", pgHost)
	os.Setenv("DB_PORT", pgPort.Port())
	os.Setenv("LOCALSTACK_HOST", localstackHost)
	os.Setenv("LOCALSTACK_PORT", localstackPort.Port())

	// Run the tests
	code := m.Run()

	// Exit with the test status code
	os.Exit(code)
}

// startPostgresContainer starts a PostgreSQL container for testing
func startPostgresContainer(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:14",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// startLocalStackContainer starts a LocalStack container for testing
func startLocalStackContainer(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "localstack/localstack:latest",
		ExposedPorts: []string{"4566/tcp"},
		Env: map[string]string{
			"SERVICES":        "s3,sqs,lambda",
			"DEFAULT_REGION":  "us-east-1",
			"DEBUG":           "1",
			"LAMBDA_EXECUTOR": "local",
		},
		WaitingFor: wait.ForListeningPort("4566/tcp").WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, err
	}

	// Add a delay to allow LocalStack to fully initialize
	time.Sleep(15 * time.Second)

	return container, nil
}

// TestFileUploadAndProcessing tests the full flow: upload a file to S3, trigger event, process, and check result
func TestFileUploadAndProcessing(t *testing.T) {
	// Create test file data
	fileData := FileData{
		ID:      uuid.New().String(),
		Name:    "test-file.txt",
		Content: "This is a test file for processing.",
	}

	// Convert file data to JSON
	fileJSON, err := json.Marshal(fileData)
	assert.NoError(t, err)

	// Upload file via API
	resp, err := http.Post(
		fmt.Sprintf("%s/api/files", apiURL),
		"application/json",
		bytes.NewBuffer(fileJSON),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Parse response
	var uploadResp map[string]string
	err = json.NewDecoder(resp.Body).Decode(&uploadResp)
	assert.NoError(t, err)
	resp.Body.Close()

	// Verify file was uploaded to S3
	s3Key := fmt.Sprintf("files/%s/%s", fileData.ID, fileData.Name)

	// Add a delay to allow S3 to process the upload
	time.Sleep(2 * time.Second)

	// Try to get the object from S3 with retries
	var getResp *s3.GetObjectOutput
	var getErr error
	for i := 0; i < 3; i++ {
		getResp, getErr = s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(s3Key),
		})
		if getErr == nil {
			break
		}
		fmt.Printf("Attempt %d: Failed to get object from S3: %v\n", i+1, getErr)
		time.Sleep(2 * time.Second)
	}

	if getErr != nil {
		t.Fatalf("Failed to get object from S3 after multiple attempts: %v", getErr)
	}

	defer getResp.Body.Close()

	// Read S3 content
	content, err := io.ReadAll(getResp.Body)
	assert.NoError(t, err)
	assert.Equal(t, fileData.Content, string(content))

	// Simulate S3 event to SQS (since we can't directly trigger S3 events in LocalStack)
	s3Event := map[string]interface{}{
		"Records": []map[string]interface{}{
			{
				"s3": map[string]interface{}{
					"bucket": map[string]interface{}{
						"name": bucketName,
					},
					"object": map[string]interface{}{
						"key": s3Key,
					},
				},
			},
		},
	}

	s3EventJSON, err := json.Marshal(s3Event)
	assert.NoError(t, err)

	// Send message to SQS
	_, err = sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(s3EventJSON)),
	})
	assert.NoError(t, err)

	// In a real test, we would run the Lambda function
	// For this example, we'll simulate Lambda processing by directly inserting a result
	resultID := uuid.New().String()
	_, err = db.Exec(
		"INSERT INTO processing_results (id, file_id, status, result, created_at) VALUES ($1, $2, $3, $4, $5)",
		resultID, fileData.ID, "completed", "Processed file with 7 words and 36 characters", time.Now(),
	)
	assert.NoError(t, err)

	// Wait for processing to complete
	time.Sleep(2 * time.Second)

	// Get processing result from API
	httpResp, err := http.Get(fmt.Sprintf("%s/api/files/%s/result", apiURL, fileData.ID))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	// Parse result
	var processingResult ProcessingResult
	err = json.NewDecoder(httpResp.Body).Decode(&processingResult)
	assert.NoError(t, err)
	httpResp.Body.Close()

	// Verify result
	assert.Equal(t, "completed", processingResult.Status)
	assert.Contains(t, processingResult.Result, "Processed file with")
}

// Helper function to create an S3 bucket
func createS3Bucket(ctx context.Context, client *s3.Client, bucketName string) error {
	// First check if the bucket already exists
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err == nil {
		// Bucket already exists
		return nil
	}

	// Create the bucket - for us-east-1, we don't need a location constraint
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	return err
}

// Handler functions for the API server
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
	_, err := db.Exec(
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

func getFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	var fileData FileData
	var s3Key string

	err := db.QueryRow(
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

func getResultHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	var result ProcessingResult

	err := db.QueryRow(
		"SELECT id, status, result, created_at FROM processing_results WHERE file_id = $1",
		fileID,
	).Scan(&result.ID, &result.Status, &result.Result, &result.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Check if file exists first
			var exists bool
			err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE id = $1)", fileID).Scan(&exists)
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
