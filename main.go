package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2/google"

	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

func generateIDToken(c *cli.Context) error {
	log.Println("getting token ...")
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		return fmt.Errorf("failed to find default credentials: %s", err)
	}

	credsMap := make(map[string]interface{})
	err = json.Unmarshal(creds.JSON, &credsMap)
	if err != nil {
		return fmt.Errorf("failed to parse credentials JSON: %s", err)
	}

	var serviceAccountID string
	var ok bool
	if serviceAccountID, ok = credsMap["client_id"].(string); !ok {
		return errors.New("failed to find service account ID")
	}

	client, err := google.DefaultClient(ctx, iamcredentials.CloudPlatformScope)
	if err != nil {
		return err
	}
	iamCredentialsClient, err := iamcredentials.New(client)
	if err != nil {
		return fmt.Errorf("failed to get iam credentials client: %s", err.Error())
	}
	generateIDTokenResponse, err := iamCredentialsClient.Projects.ServiceAccounts.GenerateIdToken(
		fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccountID),
		&iamcredentials.GenerateIdTokenRequest{
			Audience: "32555940559.apps.googleusercontent.com",
		},
	).Do()

	if err != nil {
		return fmt.Errorf("failed to generate ID token: %s", err.Error())
	}

	// parse JWT token
	parser := jwt.Parser{UseJSONNumber: true, SkipClaimsValidation: true}
	token, err := parser.Parse(generateIDTokenResponse.Token, nil)
	//jwt.Parse(generateIDTokenResponse.Token, nil)
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		unixTime, err := claims["exp"].(json.Number).Int64()
		if err != nil {
			return fmt.Errorf("failed to convert expire date: %s", err.Error())
		}
		log.Printf("token will expire on: %s", time.Unix(unixTime, 0))
	} else {
		return fmt.Errorf("failed to get claims from ID token: %s", err.Error())
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:   "gtoken",
		Usage:  "generate GCP ID token with current service account",
		Action: generateIDToken,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
