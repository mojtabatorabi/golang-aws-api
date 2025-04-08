package auth

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

var (
	cognitoClient *cognitoidentityprovider.Client
	userPoolID    string
	clientID      string
)

// InitCognito initializes the Cognito client
func InitCognito(cfg aws.Config) {
	cognitoClient = cognitoidentityprovider.NewFromConfig(cfg)

	// Get Cognito configuration from environment variables
	userPoolID = getEnv("COGNITO_USER_POOL_ID", "us-east-1_testpool")
	clientID = getEnv("COGNITO_CLIENT_ID", "1234567890abcdef")
}

// SignUp registers a new user
func SignUp(ctx context.Context, username, password, email string) (*cognitoidentityprovider.SignUpOutput, error) {
	input := &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientID),
		Username: aws.String(username),
		Password: aws.String(password),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String(email),
			},
		},
	}

	return cognitoClient.SignUp(ctx, input)
}

// ConfirmSignUp confirms a user's registration
func ConfirmSignUp(ctx context.Context, username, code string) error {
	input := &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientID),
		Username:         aws.String(username),
		ConfirmationCode: aws.String(code),
	}

	_, err := cognitoClient.ConfirmSignUp(ctx, input)
	return err
}

// SignIn authenticates a user
func SignIn(ctx context.Context, username, password string) (*cognitoidentityprovider.InitiateAuthOutput, error) {
	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME": username,
			"PASSWORD": password,
		},
		ClientId: aws.String(clientID),
	}

	return cognitoClient.InitiateAuth(ctx, input)
}

// GetUser retrieves user information
func GetUser(ctx context.Context, accessToken string) (*cognitoidentityprovider.GetUserOutput, error) {
	input := &cognitoidentityprovider.GetUserInput{
		AccessToken: aws.String(accessToken),
	}

	return cognitoClient.GetUser(ctx, input)
}

// SignOut signs out a user
func SignOut(ctx context.Context, accessToken string) error {
	input := &cognitoidentityprovider.GlobalSignOutInput{
		AccessToken: aws.String(accessToken),
	}

	_, err := cognitoClient.GlobalSignOut(ctx, input)
	return err
}

// Helper function to get environment variables
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
