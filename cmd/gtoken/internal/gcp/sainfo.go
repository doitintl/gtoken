package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2/google"
)

type ServiceAccountInfo interface {
	GetEmail() (string, error)
	GetID(context.Context) (string, error)
}

type SaInfo struct{}

func NewSaInfo() ServiceAccountInfo {
	return &SaInfo{}
}

func (sa SaInfo) GetEmail() (string, error) {
	email, err := metadata.Email("")
	if err != nil {
		return "", fmt.Errorf("failed to get default email: %s", err)
	}
	return email, nil
}

func (sa SaInfo) GetID(ctx context.Context) (string, error) {
	// handle the 'refresh token' command
	cx, cancel := context.WithCancel(ctx)
	// cancel current context on exit
	defer cancel()
	creds, err := google.FindDefaultCredentials(cx)
	if err != nil {
		return "", fmt.Errorf("failed to find default credentials: %s", err)
	}
	credsMap := make(map[string]interface{})
	err = json.Unmarshal(creds.JSON, &credsMap)
	if err != nil {
		return "", fmt.Errorf("failed to parse credentials JSON: %s", err)
	}
	if id, ok := credsMap["client_id"].(string); ok {
		return id, nil
	}
	return "", errors.New("failed to find service account ID")
}
