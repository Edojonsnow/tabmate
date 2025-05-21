package auth

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type ClaimsPage struct {
    AccessToken string
    Claims      jwt.MapClaims
}

type UserInfo struct {
    Sub            string `json:"sub"`
    Email          string `json:"email"`
    EmailVerified  bool   `json:"email_verified"`
    Name           string `json:"name"`
    PhoneNumber    string `json:"phone_number"`
    PhoneVerified  bool   `json:"phone_number_verified"`
}

var (
    clientID     string
    clientSecret string
    redirectURL  string
    issuerURL    string
    provider     *oidc.Provider
    OAuth2Config oauth2.Config
    cognitoClient *cognitoidentityprovider.Client
)

func init() {
	var err error

	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
    // Load required environment variables
    clientID = os.Getenv("COGNITO_CLIENT_ID")
    clientSecret = os.Getenv("COGNITO_CLIENT_SECRET")
    redirectURL = os.Getenv("REDIRECT_URL")
    issuerURL = os.Getenv("ISSUER_URL")

    // Validate required environment variables
    requiredEnvVars := map[string]string{
        "COGNITO_CLIENT_ID": clientID,
        "COGNITO_CLIENT_SECRET": clientSecret,
        "REDIRECT_URL": redirectURL,
        "ISSUER_URL": issuerURL,
    }

    for name, value := range requiredEnvVars {
        if value == "" {
            log.Fatalf("Required environment variable %s is not set", name)
        }
    }

    // Initialize AWS SDK
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        log.Fatalf("Unable to load SDK config: %v", err)
    }

    // Initialize Cognito client
    cognitoClient = cognitoidentityprovider.NewFromConfig(cfg)

    // Log the configuration (excluding sensitive data)
    log.Printf("Initializing OIDC provider with issuer URL: %s", issuerURL)
    log.Printf("Redirect URL: %s", redirectURL)

   
    // Initialize OIDC provider
    provider, err = oidc.NewProvider(context.Background(), issuerURL)
    if err != nil {
        log.Fatalf("Failed to create OIDC provider: %v", err)
    }

    // Set up OAuth2 config
    OAuth2Config = oauth2.Config{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        RedirectURL:  redirectURL,
        Endpoint:     provider.Endpoint(),
        Scopes:       []string{oidc.ScopeOpenID, "phone", "openid", "email", "profile"},
    }

    log.Println("OAuth2 configuration initialized successfully")
}

// GetUserInfo retrieves user information from the ID token
func GetUserInfo(ctx context.Context, idToken string) (*UserInfo, error) {
    verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
    
    // Parse and verify the ID token
    token, err := verifier.Verify(ctx, idToken)
    if err != nil {
        return nil, fmt.Errorf("failed to verify ID token: %v", err)
    }

    // Extract claims
    var claims struct {
        Sub               string `json:"sub"`
        Email            string `json:"email"`
        EmailVerified    bool   `json:"email_verified"`
        Name             string `json:"name"`
        PhoneNumber      string `json:"phone_number"`
        PhoneVerified    bool   `json:"phone_number_verified"`
    }

    if err := token.Claims(&claims); err != nil {
        return nil, fmt.Errorf("failed to parse claims: %v", err)
    }

    return &UserInfo{
        Sub:           claims.Sub,
        Email:         claims.Email,
        EmailVerified: claims.EmailVerified,
        Name:          claims.Name,
        PhoneNumber:   claims.PhoneNumber,
        PhoneVerified: claims.PhoneVerified,
    }, nil
}


// ListUsers retrieves all users from the Cognito User Pool
func ListUsers(ctx context.Context) ([]types.UserType, error) {
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")
	if userPoolID == "" {
		return nil, fmt.Errorf("COGNITO_USER_POOL_ID environment variable is not set")
	}

	var allUsers []types.UserType
	var paginationToken *string

	for {
		input := &cognitoidentityprovider.ListUsersInput{
			UserPoolId: &userPoolID,
		}

		if paginationToken != nil {
			input.PaginationToken = paginationToken
		}

		output, err := cognitoClient.ListUsers(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %v", err)
		}

		allUsers = append(allUsers, output.Users...)

		if output.PaginationToken == nil {
			break
		}
		paginationToken = output.PaginationToken
	}

	return allUsers, nil
}

// GetCognitoClient returns the Cognito client instance
func GetCognitoClient() *cognitoidentityprovider.Client {
	return cognitoClient
}

// GetClientID returns the Cognito client ID
func GetClientID() string {
	return clientID
}

// GetClientSecret returns the Cognito client secret
func GetClientSecret() string {
	return clientSecret
}