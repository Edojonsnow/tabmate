package client

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"tabmate/internals/auth"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

var (
	cognitoclientId = auth.GetClientID()
	clientSecret    = auth.GetClientSecret()
)

// computeSecretHash computes the SECRET_HASH for Cognito operations
func computeSecretHash(username string) string {
	message := username + cognitoclientId
	key := []byte(clientSecret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// SignUp signs up a user with Amazon Cognito.
func (actor CognitoActions) SignUp(ctx context.Context, userName string, password string, userEmail string, phoneNumber string) (bool, error) {
	confirmed := false
	output, err := actor.CognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(cognitoclientId),
		Password: aws.String(password),
		Username: aws.String(userName),
		SecretHash: aws.String(computeSecretHash(userName)),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(userEmail)},
			{Name: aws.String("name"), Value: aws.String(userName)},
			{Name: aws.String("phone_number"), Value: aws.String(phoneNumber)},
		},
	})
	if err != nil {
		var invalidPassword *types.InvalidPasswordException
		if errors.As(err, &invalidPassword) {
			log.Println(*invalidPassword.Message)
		} else {
			log.Printf("Couldn't sign up user %v. Here's why: %v\n", userName, err)
		}
	} else {
		confirmed = output.UserConfirmed
	}
	return confirmed, err
}

func (actor CognitoActions) SignIn(ctx context.Context, userName string, password string) (*types.AuthenticationResultType, error) {
	var authResult *types.AuthenticationResultType
	output, err := actor.CognitoClient.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: "USER_PASSWORD_AUTH",
		ClientId: aws.String(cognitoclientId),
		AuthParameters: map[string]string{
			"USERNAME":    userName,
			"PASSWORD":    password,
			"SECRET_HASH": computeSecretHash(userName),
		},
	})
	if err != nil {
		var resetRequired *types.PasswordResetRequiredException
		if errors.As(err, &resetRequired) {
			log.Println(*resetRequired.Message)
		} else {
			log.Printf("Couldn't sign in user %v. Here's why: %v\n", userName, err)
		}
	} else {
		authResult = output.AuthenticationResult
	}
	return authResult, err
}

func (actor CognitoActions) ForgotPassword(ctx context.Context, userName string) (*types.CodeDeliveryDetailsType, error) {
	output, err := actor.CognitoClient.ForgotPassword(ctx, &cognitoidentityprovider.ForgotPasswordInput{
		ClientId: aws.String(cognitoclientId),
		Username: aws.String(userName),
	})
	if err != nil {
		log.Printf("Couldn't start password reset for user '%v'. Here;s why: %v\n", userName, err)
	}
	return output.CodeDeliveryDetails, err
}

func (actor CognitoActions) ConfirmForgotPassword(ctx context.Context, code string, userName string, password string) error {
	_, err := actor.CognitoClient.ConfirmForgotPassword(ctx, &cognitoidentityprovider.ConfirmForgotPasswordInput{
		ClientId:         aws.String(cognitoclientId),
		ConfirmationCode: aws.String(code),
		Password:         aws.String(password),
		Username:         aws.String(userName),
	})
	if err != nil {
		var invalidPassword *types.InvalidPasswordException
		if errors.As(err, &invalidPassword) {
			log.Println(*invalidPassword.Message)
		} else {
			log.Printf("Couldn't confirm user %v. Here's why: %v", userName, err)
		}
	}
	return err
}