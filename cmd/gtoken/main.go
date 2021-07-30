package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/doitintl/gtoken/internal/aws"
	"github.com/doitintl/gtoken/internal/gcp"
	"github.com/urfave/cli/v2"
)

var (
	// Version contains the current version.
	Version = "dev"
	// BuildDate contains a string with the build date.
	BuildDate = "unknown"
)

const awsFileVariablesPathEnv = "AWS_FILE_VARIABLES_PATH"

func generateEnvFileVariables(fileVariables aws.FileVariables) error {
	path, ok := os.LookupEnv(awsFileVariablesPathEnv)
	if !ok {
		return fmt.Errorf("no-webhook: could not read AWS file variables path from %s", awsFileVariablesPathEnv)
	}
	return fileVariables.GenerateToFile(path)
}

func generateIDToken(ctx context.Context, sa gcp.ServiceAccountInfo, idToken gcp.Token, file string, refresh bool) error {
	// find out active Service Account, first by ID
	serviceAccount, err := sa.GetID(ctx)
	if err != nil {
		log.Printf("failed to get service account, fallback to metadata email: %s\n", err)
		// fallback: try to get Service Account email from metadata server
		serviceAccount, err = sa.GetEmail()
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
		case <-ctx.Done():
			return nil // avoid goroutine leak
		case <-timer:
			// generate ID token
			token, err := idToken.Generate(ctx, serviceAccount)
			if err != nil {
				return err
			}
			// write generated token to file or stdout
			err = idToken.WriteToFile(token, file)
			if err != nil {
				return err
			}
			// auto-refresh enabled
			if refresh {
				// get token duration
				duration, err = idToken.GetDuration(token)
				if err != nil {
					return err
				}
				// reduce duration by 30s
				duration -= 30 * time.Second
				log.Printf("refreshing token in %s", duration)
				// reset timer
				timer = time.NewTimer(duration).C
			} else {
				return nil // avoid goroutine leak
			}
		}
	}
}

func generateIDTokenCmd(c *cli.Context) error {
	if c.Bool("no-webhook") {
		if err := generateEnvFileVariables(aws.NewAWSEnvFileVariables()); err != nil {
			return err
		}
	}
	if err := generateIDToken(handleSignals(), gcp.NewSaInfo(), gcp.NewIDToken(), c.String("file"), c.Bool("refresh")); err != nil {
		return err
	}
	return nil
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
				Value: false,
				Usage: "auto refresh ID token before it expires",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "write ID token into file (stdout, if not specified)",
			},
			&cli.BoolFlag{
				Name:  "no-webhook",
				Value: false,
				Usage: "compatibility flag for Kubernetes clusters where mutating webhooks cannot be installed, such as GKE Autopilot",
			},
		},
		Name:    "gtoken",
		Usage:   "generate ID token with current Google Cloud service account",
		Action:  generateIDTokenCmd,
		Version: Version,
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("gtoken %s\n", Version)
		fmt.Printf("  Build date: %s\n", BuildDate)
		fmt.Printf("  Built with: %s\n", runtime.Version())
	}
	// print version
	log.Printf("running gtoken version: %s\n", app.Version)

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
