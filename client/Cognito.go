package client

import (
	"context"
	"errors"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)


type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

// SignUp signs up a user with Amazon Cognito.
func (actor CognitoActions) SignUp(ctx context.Context, clientId string, userName string, password string, userEmail string) (bool, error) {
	confirmed := false
	output, err := actor.CognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientId),
		Password: aws.String(password),
		Username: aws.String(userName),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(userEmail)},
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

func (actor CognitoActions) SignIn(ctx context.Context, clientId string, userName string, password string) (*types.AuthenticationResultType, error) {
	var authResult *types.AuthenticationResultType
	output, err := actor.CognitoClient.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       "USER_PASSWORD_AUTH",
		ClientId:       aws.String(clientId),
		AuthParameters: map[string]string{"USERNAME": userName, "PASSWORD": password},
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

func (actor CognitoActions) ForgotPassword(ctx context.Context, clientId string, userName string) (*types.CodeDeliveryDetailsType, error) {
	output, err := actor.CognitoClient.ForgotPassword(ctx, &cognitoidentityprovider.ForgotPasswordInput{
		ClientId: aws.String(clientId),
		Username: aws.String(userName),
	})
	if err != nil {
		log.Printf("Couldn't start password reset for user '%v'. Here;s why: %v\n", userName, err)
	}
	return output.CodeDeliveryDetails, err
}


func (actor CognitoActions) ConfirmForgotPassword(ctx context.Context, clientId string, code string, userName string, password string) error {
	_, err := actor.CognitoClient.ConfirmForgotPassword(ctx, &cognitoidentityprovider.ConfirmForgotPasswordInput{
		ClientId:         aws.String(clientId),
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