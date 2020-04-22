package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"google.golang.org/api/iamcredentials/v1"
)

const (
	// default aud
	defaultAud = "gtoken/sts/assume-role-with-web-identity"
)

type Token interface {
	Generate(context.Context, string) (string, error)
	GetDuration(string) (time.Duration, error)
	WriteToFile(string, string) error
}

type IDToken struct{}

func NewIDToken() Token {
	return &IDToken{}
}

func (IDToken) Generate(ctx context.Context, serviceAccount string) (string, error) {
	log.Println("generating a new ID token")
	iamCredentialsClient, err := iamcredentials.NewService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get iam credentials client: %s", err.Error())
	}
	generateIDTokenResponse, err := iamCredentialsClient.Projects.ServiceAccounts.GenerateIdToken(
		fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccount),
		&iamcredentials.GenerateIdTokenRequest{
			Audience:     defaultAud,
			IncludeEmail: true,
		},
	).Do()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID token: %s", err.Error())
	}
	log.Println("successfully generated ID token")
	return generateIDTokenResponse.Token, nil
}

func (IDToken) GetDuration(jwtToken string) (time.Duration, error) {
	// parse JWT token
	parser := jwt.Parser{UseJSONNumber: true, SkipClaimsValidation: true}
	token, _, err := parser.ParseUnverified(jwtToken, jwt.MapClaims{})
	if err != nil {
		return 0, fmt.Errorf("failed to parse jwtToken: %s", err.Error())
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		var unixTime int64
		unixTime, err = claims["exp"].(json.Number).Int64()
		if err != nil {
			return 0, fmt.Errorf("failed to convert expire date: %s", err.Error())
		}
		return time.Until(time.Unix(unixTime, 0)), nil
	}
	return 0, fmt.Errorf("failed to get claims from ID token: %s", err.Error())
}

func (IDToken) WriteToFile(token, fileName string) error {
	// this is a slice of io.Writers we will write the file to
	var writers []io.Writer

	// if no file provided
	if fileName == "" {
		writers = append(writers, os.Stdout)
	}

	// if DestFile was provided, lets try to create it and add to the writers
	if len(fileName) > 0 {
		file, err := os.Create(fileName)
		if err != nil {
			return fmt.Errorf("failed to create token file: %s; error: %s", fileName, err.Error())
		}
		writers = append(writers, file)
		defer file.Close()
	}
	// MultiWriter(io.Writer...) returns a single writer which multiplexes its
	// writes across all of the writers we pass in.
	dest := io.MultiWriter(writers...)
	// write to dest the same way as before, copying from the Body
	if _, err := io.WriteString(dest, token); err != nil {
		return fmt.Errorf("failed to write token: %s", err.Error())
	}
	return nil
}
