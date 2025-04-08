// lambda/main.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var (
	s3Client   *s3.Client
	bucketName string
	db         *sql.DB
)

// S3Event represents the S3 event that triggers this Lambda
type S3Event struct {
	Records []struct {
		S3 struct {
			Bucket struct {
				Name string `json:"name"`
			} `json:"bucket"`
			Object struct {
				Key string `json:"key"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

// ProcessingResult represents the result of file processing
type ProcessingResult struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

func init() {
	// Set up AWS configuration
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if os.Getenv("ENV") == "local" {
			return aws.Endpoint{
				URL:           "http://localhost:4566",
				SigningRegion: "us-east-1",
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
	)

	if os.Getenv("ENV") == "local" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
	}

	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)

	// Set bucket name
	bucketName = os.Getenv("S3_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "my-test-bucket"
	}

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

	var dbErr error
	db, dbErr = sql.Open("postgres", dbInfo)
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
}

func HandleSQSEvent(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, message := range sqsEvent.Records {
		// Parse the S3 event from the SQS message
		var s3Event S3Event
		if err := json.Unmarshal([]byte(message.Body), &s3Event); err != nil {
			log.Printf("Error parsing S3 event: %v", err)
			continue
		}

		// Process each S3 record
		for _, record := range s3Event.Records {
			bucketName := record.S3.Bucket.Name
			objectKey := record.S3.Object.Key

			// Get file ID from the object key (format: "files/{fileID}/{filename}")
			parts := strings.Split(objectKey, "/")
			if len(parts) < 2 {
				log.Printf("Invalid object key format: %s", objectKey)
				continue
			}
			fileID := parts[1]

			// Get file from S3
			result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(objectKey),
			})
			if err != nil {
				log.Printf("Error getting object from S3: %v", err)
				continue
			}
			defer result.Body.Close()

			// Process the file content (simple example)
			content, err := io.ReadAll(result.Body)
			if err != nil {
				log.Printf("Error reading object content: %v", err)
				continue
			}

			fileContent := string(content)

			// Simple processing - count words and characters
			words := len(strings.Fields(fileContent))
			chars := len(fileContent)

			processedResult := fmt.Sprintf("Processed file with %d words and %d characters", words, chars)

			// Store result in database
			processingResult := ProcessingResult{
				ID:        uuid.New().String(),
				FileID:    fileID,
				Status:    "completed",
				Result:    processedResult,
				CreatedAt: time.Now(),
			}

			_, err = db.Exec(
				"INSERT INTO processing_results (id, file_id, status, result, created_at) VALUES ($1, $2, $3, $4, $5)",
				processingResult.ID, processingResult.FileID, processingResult.Status, processingResult.Result, processingResult.CreatedAt,
			)
			if err != nil {
				log.Printf("Error saving processing result: %v", err)
				continue
			}

			log.Printf("Successfully processed file %s", objectKey)
		}
	}

	return nil
}

func main() {
	lambda.Start(HandleSQSEvent)
}
