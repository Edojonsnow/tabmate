package auth

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkjwt "github.com/clerk/clerk-sdk-go/v2/jwt"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		log.Fatal("CLERK_SECRET_KEY is not set")
	}

	clerk.SetKey(secretKey)
	log.Println("Clerk auth initialized")
}

// ClerkUserInfo holds identity extracted from a verified Clerk session token.
type ClerkUserInfo struct {
	Sub   string // Clerk user ID (e.g. user_2abc...)
	Email string
	Name  string
}

// VerifyClerkToken verifies a Clerk session token and returns the caller's identity.
func VerifyClerkToken(tokenString string) (*ClerkUserInfo, error) {
	claims, err := clerkjwt.Verify(context.Background(), &clerkjwt.VerifyParams{
		Token: tokenString,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Clerk token: %w", err)
	}

	info := &ClerkUserInfo{Sub: claims.Subject}

	// Fetch email/name from Clerk API.
	details, err := fetchClerkUser(info.Sub)
	if err != nil {
		log.Printf("Warning: could not fetch Clerk user details for %s: %v", info.Sub, err)
	} else {
		info.Email = details.Email
		info.Name = details.Name
	}

	return info, nil
}

func fetchClerkUser(userID string) (*ClerkUserInfo, error) {
	u, err := clerkuser.Get(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	info := &ClerkUserInfo{Sub: userID}

	// Resolve primary email.
	for _, e := range u.EmailAddresses {
		if u.PrimaryEmailAddressID != nil && e.ID == *u.PrimaryEmailAddressID {
			info.Email = e.EmailAddress
			break
		}
	}
	if info.Email == "" && len(u.EmailAddresses) > 0 {
		info.Email = u.EmailAddresses[0].EmailAddress
	}

	// Build display name.
	parts := []string{}
	if u.FirstName != nil && *u.FirstName != "" {
		parts = append(parts, *u.FirstName)
	}
	if u.LastName != nil && *u.LastName != "" {
		parts = append(parts, *u.LastName)
	}
	info.Name = strings.Join(parts, " ")

	return info, nil
}
