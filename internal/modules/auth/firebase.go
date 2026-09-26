package auth

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type firebaseVerifier struct {
	client *auth.Client
}

type unavailableIdentity struct{}

func (unavailableIdentity) Verify(context.Context, string) (Identity, error) {
	return Identity{}, apperror.ErrUnavailable
}

func OpenIdentity(ctx context.Context, projectID, credentialsFile, credentialsJSON string) (tokenVerifier, error) {
	if credentialsJSON != "" {
		return newFirebase(ctx, projectID, option.WithCredentialsJSON([]byte(credentialsJSON)))
	}
	if credentialsFile == "" {
		return unavailableIdentity{}, nil
	}
	return newFirebase(ctx, projectID, option.WithCredentialsFile(credentialsFile))
}

func newFirebase(ctx context.Context, projectID string, opt option.ClientOption) (tokenVerifier, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, opt)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: firebase auth: %w", err)
	}
	return &firebaseVerifier{client: client}, nil
}

func (v *firebaseVerifier) Verify(ctx context.Context, idToken string) (Identity, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Identity{}, mapFirebase(err)
	}
	return identityFrom(token), nil
}

func mapFirebase(err error) error {
	if auth.IsIDTokenExpired(err) {
		return apperror.Unauthorized("firebase token expired")
	}
	if auth.IsIDTokenInvalid(err) {
		return apperror.Unauthorized("firebase token audience is invalid")
	}
	return fmt.Errorf("auth: verify firebase token: %w", err)
}

func identityFrom(token *auth.Token) Identity {
	return Identity{
		UID:   token.UID,
		Email: claimString(token.Claims, "email"),
		Name:  claimString(token.Claims, "name"),
	}
}

func claimString(claims map[string]interface{}, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}
