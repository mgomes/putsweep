package auth

import (
	"context"
	"net/http"

	putio "github.com/putdotio/go-putio"
	"golang.org/x/oauth2"
)

func NewClient(token string) *putio.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	return putio.NewClient(oauthClient)
}

func ValidateToken(token string) (bool, error) {
	client := NewClient(token)
	ctx := context.Background()

	_, err := client.Account.Info(ctx)
	if err != nil {
		if isUnauthorized(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	if httpErr, ok := err.(*putio.ErrorResponse); ok {
		return httpErr.Response.StatusCode == http.StatusUnauthorized
	}
	return false
}
