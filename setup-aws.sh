#!/bin/bash
# setup-aws.sh - Setup AWS resources on LocalStack

# Wait for LocalStack to be ready
echo "Waiting for LocalStack to be ready..."
while ! nc -z localhost 4566; do
  sleep 1
done

# Configure AWS CLI to use LocalStack
aws configure set aws_access_key_id test
aws configure set aws_secret_access_key test
aws configure set region us-east-1
aws configure set output json

# Create S3 bucket
echo "Creating S3 bucket..."
aws --endpoint-url=http://localhost:4566 s3 mb s3://my-test-bucket

# Create SQS queue
echo "Creating SQS queue..."
aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name my-queue

# Create Lambda function (assuming the Lambda code is already built)
echo "Creating Lambda function..."
aws --endpoint-url=http://localhost:4566 lambda create-function \
  --function-name file-processor \
  --runtime go1.x \
  --handler lambda \
  --zip-file fileb:///var/task/lambda.zip \
  --role arn:aws:iam::000000000000:role/lambda-role

# Set up S3 event notification
echo "Setting up S3 event notification..."
aws --endpoint-url=http://localhost:4566 s3api put-bucket-notification-configuration \
  --bucket my-test-bucket \
  --notification-configuration '{
    "QueueConfigurations": [
      {
        "QueueArn": "arn:aws:sqs:us-east-1:000000000000:my-queue",
        "Events": ["s3:ObjectCreated:*"]
      }
    ]
  }'

# Set up SQS event source mapping for Lambda
echo "Setting up SQS event source mapping for Lambda..."
aws --endpoint-url=http://localhost:4566 lambda create-event-source-mapping \
  --function-name file-processor \
  --batch-size 1 \
  --event-source-arn arn:aws:sqs:us-east-1:000000000000:my-queue

echo "AWS resources setup complete!"