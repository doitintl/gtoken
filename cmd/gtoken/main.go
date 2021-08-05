package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/doitintl/gtoken/internal/aws"
	"github.com/doitintl/gtoken/internal/gcp"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var (
	// Version contains the current version.
	Version = "dev"
	// BuildDate contains a string with the build date.
	BuildDate = "unknown"
)

func generateEnvFileVariables(fileVariables aws.FileVariables, filePath string) error {
	return fileVariables.GenerateToFile(filePath)
}

//nolint: funlen, gocyclo
func handleIDToken(wrappingCtx context.Context, sa gcp.ServiceAccountInfo, idToken gcp.Token, file string, refresh, daemon bool) error {
	g, ctx := errgroup.WithContext(wrappingCtx)

	// find out active Service Account, first by ID
	serviceAccount, err := sa.GetID(wrappingCtx)
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

	tokenFileDone := make(chan bool)

	g.Go(func() error {
		for {
			// wait for next timer tick or cancel
			select {
			case <-ctx.Done():
				return ctx.Err()
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
				tokenFileDone <- true
				close(tokenFileDone)

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
	})

	if daemon {
		log.Printf("Starting daemon mode...")
		ok, index := sliceContains(os.Args, "--")
		if !ok {
			return fmt.Errorf("app invocation and arguments should be prefixed by -- , e.g. gtoken --daemon -- myapp foo")
		}
		command := os.Args[index:]
		binary, err := exec.LookPath(command[0])
		if err != nil {
			return err
		}
		cmd := exec.Command(binary, command[1:]...)
		cmd.Env = os.Environ()
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		// Prepare to relay all received Unix signals to forked application process
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs)

		g.Go(func() error {
			// Wait for the token file to be written to disk
			<-tokenFileDone

			err = cmd.Start()
			if err != nil {
				return err
			}
			err = cmd.Wait()
			close(sigs)
			return err
		})

		// Handle Unix signals relaying
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-sigs:
				for sig := range sigs {
					// We don't want to signal a non-running process.
					if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
						break
					}
					err := cmd.Process.Signal(sig)
					if err != nil {
						log.Printf("failed to signal process with %s: %v", sig, err)
					} else {
						log.Printf("received signal: %s", sig)
					}
				}
			}
			return nil
		})
	}
	return g.Wait()
}

// sliceContains checks if a string is present in a string slice, and its position in the slice.
func sliceContains(s []string, e string) (bool, int) {
	for i, a := range s {
		if a == e {
			return true, i
		}
	}
	return false, 0
}

func handler(c *cli.Context) error {
	if c.Bool("daemon") {
		// co-locating the variable files alongside the token file
		tokenFilePathSlice := strings.Split(c.String("file"), "/")
		filePath := strings.Join(tokenFilePathSlice[0:len(tokenFilePathSlice)-2], "/") + "/variables-file"
		if err := generateEnvFileVariables(aws.NewAWSEnvFileVariables(), filePath); err != nil {
			return err
		}
	}
	//nolint:funlen
	if err := handleIDToken(
		handleSignals(),
		gcp.NewSaInfo(),
		gcp.NewIDToken(),
		c.String("file"),
		c.Bool("refresh"),
		c.Bool("daemon")); err != nil {
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
				Name:  "daemon",
				Value: false,
				Usage: "compatibility flag for Kubernetes clusters where mutating webhooks cannot be installed, such as GKE Autopilot.",
			},
		},
		Name:    "gtoken",
		Usage:   "generate ID token with current Google Cloud service account",
		Action:  handler,
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
