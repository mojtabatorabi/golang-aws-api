1- $docker compose -u -d 
2 - test:
    $cd tests
    $go test -v

3- show files in database
          $ cd /cmd/report
          $ go run main.go

-------------------------------------------
1. First, let's look at the `cmd` directory:

### Project Structure Overview

This is a Go-based AWS API project that uses LocalStack for local development. Here's a breakdown of each component:

#### 1. Main Application Files
- **cmd/report/main.go**: A utility tool to report on files stored in the database
- **database/db.go**: Database connection management
- **database/files.go**: File-related database operations
- **database/processing.go**: Processing-related database operations

#### 2. AWS Lambda Integration
- **lambda/main.go**: Lambda function code for processing files
- **lambda/Dockerfile.lambda**: Docker configuration for the Lambda function

#### 3. Infrastructure Files
- **docker-compose.yml**: Defines the services (API, Lambda, PostgreSQL, LocalStack)
- **Dockerfile**: Main application Docker configuration
- **setup-aws.sh**: Script to initialize AWS resources in LocalStack

#### 4. Testing
- **tests/integration_test.go**: Integration tests for the entire system

#### 5. Dependency Management
- **go.mod**: Go module definition and dependencies
- **go.sum**: Dependency checksums

### Key Components

1. **API Service**: Handles file uploads and processing requests
2. **Lambda Service**: Processes files asynchronously
3. **PostgreSQL Database**: Stores file metadata and processing results
4. **LocalStack**: Provides local AWS services (S3, SQS, Lambda)
5. **Reporting Tool**: Utility to check database contents

### How It Works

1. Files are uploaded through the API
2. File metadata is stored in PostgreSQL
3. File content is stored in S3
4. Processing requests are sent to SQS
5. Lambda function processes the files
6. Results are stored back in the database

This architecture allows for scalable file processing with AWS services, all running locally for development using LocalStack.

explain any specific component in more detail:
---------------



### 1. Root Directory Files

1. **go.mod** - Go module definition file that lists all dependencies
2. **go.sum** - Dependency checksums for security
3. **docker-compose.yml** - Defines all services (API, Lambda, PostgreSQL, LocalStack)
4. **setup-aws.sh** - Script to initialize AWS resources in LocalStack
5. **Dockerfile** - Main application Docker configuration

### 2. Let's check the cmd directory:



### Detailed File Explanations

#### 1. Root Directory
1. **go.mod**
   - Defines the Go module and its dependencies
   - Contains version information for all external packages

2. **go.sum**
   - Contains cryptographic checksums of dependencies
   - Ensures dependency integrity

3. **docker-compose.yml**
   - Defines four services:
     - `api`: Main application service
     - `lambda`: AWS Lambda function service
     - `postgres`: PostgreSQL database
     - `localstack`: Local AWS services emulator

4. **setup-aws.sh**
   - Initializes AWS resources in LocalStack
   - Creates S3 buckets, SQS queues, and Lambda functions

5. **Dockerfile**
   - Builds the main application container
   - Sets up the Go environment and dependencies

#### 2. Command Line Tools (cmd/)
1. **cmd/report/main.go**
   - Utility tool to report on database contents
   - Shows file counts and details
   - Connects to PostgreSQL and displays file information

#### 3. Database Package (database/)
1. **database/db.go**
   - Manages database connection
   - Initializes PostgreSQL connection
   - Provides connection pool management

2. **database/files.go**
   - Handles file-related database operations
   - Functions for saving and retrieving file metadata
   - Manages file records in PostgreSQL

3. **database/processing.go**
   - Manages file processing status
   - Handles processing results storage
   - Tracks file processing state

#### 4. Lambda Function (lambda/)
1. **lambda/main.go**
   - AWS Lambda function implementation
   - Processes uploaded files
   - Handles file content analysis

2. **lambda/Dockerfile.lambda**
   - Builds the Lambda function container
   - Configures Lambda-specific environment

#### 5. Tests (tests/)
1. **tests/integration_test.go**
   - End-to-end integration tests
   - Tests the entire system workflow
   - Verifies file upload, processing, and retrieval

### System Flow
1. User uploads a file through the API
2. File metadata is stored in PostgreSQL
3. File content is stored in S3
4. Processing request is sent to SQS
5. Lambda function processes the file
6. Results are stored back in the database
7. User can retrieve file and processing results

This architecture provides:
- Scalable file processing
- Asynchronous operations
- Persistent storage
- Local development environment
- Complete testing coverage

***cognito***
curl -X POST http://localhost:8080/api/auth/signin \
     -H "Content-Type: application/json" \
     -d '{"username": "testuser3", "password": "testpass123"}'
{"access_token":"HlHxSdxbzk/i5V5M6+G5evPSMpF7vr8ahcmOxhtJfDM=","id_token":"HlHxSdxbzk/i5V5M6+G5evPSMpF7vr8ahcmOxhtJfDM="}




sudo kill -9 $(sudo lsof -t -i:8080)


export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=postgres
export ENV=local
export S3_BUCKET_NAME=my-test-bucket
export LOCALSTACK_HOST=localhost
export LOCALSTACK_PORT=4566

docker compose up -d
*singup user*
   curl -X POST http://localhost:8080/api/auth/signup \
     -H "Content-Type: application/json" \
     -d '{"username": "testuser6", "password": "testpass123", "email": "test@example.com"}'
*confirm*
   curl -X POST http://localhost:8080/api/auth/confirm \
     -H "Content-Type: application/json" \
     -d '{"username": "testuser6", "code": "123456"}'
*signin*
   curl -X POST http://localhost:8080/api/auth/signin \
     -H "Content-Type: application/json" \
     -d '{"username": "testuser6", "password": "testpass123"}'
*upload file*
   curl -X POST http://localhost:8080/api/files \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_TOKEN_HERE" \
     -d '{"name": "test767676.txt", "content": "Hello, World!"}'

   curl -X POST http://localhost:8080/api/files \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer JoF4FJuhO87dEQN7vWhmBTFE+l/sZ0fr4jiihct5m5w=" \
     -d '{"name": "test76767676767676-1.txt", "content": "Hello, World!"}'
