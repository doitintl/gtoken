package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/dgrijalva/jwt-go"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2/google"

	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

var (
	// main context
	mainCtx context.Context
	// Version contains the current version.
	Version = "dev"
	// BuildDate contains a string with the build date.
	BuildDate = "unknown"
)

func getServiceAccountID(ctx context.Context) (string, error) {
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

func getServiceAccountEmail() (string, error) {
	email, err := metadata.Email("")
	if err != nil {
		return "", fmt.Errorf("failed to get default email: %s", err)
	}
	return email, nil
}

func generateIDToken(ctx context.Context, serviceAccount string) (string, error) {
	log.Println("generating a new ID token")

	// handle the 'refresh token' command
	cx, cancel := context.WithCancel(ctx)
	// cancel current context on exit
	defer cancel()

	client, err := google.DefaultClient(cx, iamcredentials.CloudPlatformScope)
	if err != nil {
		return "", err
	}
	iamCredentialsClient, err := iamcredentials.New(client)
	if err != nil {
		return "", fmt.Errorf("failed to get iam credentials client: %s", err.Error())
	}
	generateIDTokenResponse, err := iamCredentialsClient.Projects.ServiceAccounts.GenerateIdToken(
		fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccount),
		&iamcredentials.GenerateIdTokenRequest{
			Audience: "32555940559.apps.googleusercontent.com",
		},
	).Do()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID token: %s", err.Error())
	}
	log.Println("successfully generated ID token")
	return generateIDTokenResponse.Token, nil
}

func getTokenDuration(jwtToken string) (time.Duration, error) {
	// parse JWT token
	parser := jwt.Parser{UseJSONNumber: true, SkipClaimsValidation: true}
	token, err := parser.Parse(jwtToken, nil)
	//jwt.Parse(generateIDTokenResponse.Token, nil)
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		unixTime, err := claims["exp"].(json.Number).Int64()
		if err != nil {
			return 0, fmt.Errorf("failed to convert expire date: %s", err.Error())
		}
		return time.Unix(unixTime, 0).Sub(time.Now()), nil
	}
	return 0, fmt.Errorf("failed to get claims from ID token: %s", err.Error())
}

func writeToken(token string, fileName string) error {
	// this is a slice of io.Writers we will write the file to
	var writers []io.Writer

	// if no file provided
	if len(fileName) == 0 {
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

func generateIDTokenCmd(c *cli.Context) error {
	// find out active Service Account, first by ID
	serviceAccount, err := getServiceAccountID(mainCtx)
	if err != nil {
		// fallback: try to get Service Account email from metadata server
		serviceAccount, err = getServiceAccountEmail()
	}
	if err != nil {
		return err
	}
	// initial duration to 1ms
	duration := time.Millisecond
	timer := time.NewTimer(duration).C
	for {
		// wait for next timer tick or cancel
		select {
		case <-mainCtx.Done():
			return nil // avoid goroutine leak
		case <-timer:
			// generate ID token
			token, err := generateIDToken(mainCtx, serviceAccount)
			if err != nil {
				return err
			}
			// write generated token to file or stdout
			err = writeToken(token, c.String("file"))
			if err != nil {
				return err
			}
			// auto-refresh enabled
			if c.Bool("refresh") {
				// get token duration
				duration, err = getTokenDuration(token)
				// debug reduce duration
				if err != nil {
					return err
				}
				// reduce duration by 30s
				duration = duration - 30*time.Second
				log.Printf("refreshing token in %s", duration)
				// reset timer
				timer = time.NewTimer(duration).C
			} else {
				return nil // avoid goroutine leak
			}
		}
	}
}

func init() {
	// handle termination signal
	mainCtx = handleSignals()
}

func handleSignals() context.Context {
	// Graceful shut-down on SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// create cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		sid := <-sig
		log.Printf("received signal: %d\n", sid)
		log.Println("canceling token refresh ...")
	}()

	return ctx
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "refresh",
				Value: true,
				Usage: "auto refresh ID token before it expires",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "write ID token into file",
			},
		},
		Name:    "gtoken",
		Usage:   "generate GCP ID token with current service account",
		Action:  generateIDTokenCmd,
		Version: Version,
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("gtoken %s\n", Version)
		fmt.Printf("  Build date: %s\n", BuildDate)
		fmt.Printf("  Built with: %s\n", runtime.Version())
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
