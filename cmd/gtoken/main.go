package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/doitintl/gtoken/internal/gcp"

	"github.com/urfave/cli/v2"
)

var (
	// Version contains the current version.
	Version = "dev"
	// BuildDate contains a string with the build date.
	BuildDate = "unknown"

	ServerAddr = ":2014"
)

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
			log.Println("context canceled. returning....")
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

func startServerAndGenerator(ctx context.Context, saInfo gcp.ServiceAccountInfo, token gcp.Token, file string, refresh bool) error {
	mainCtx, cancel := context.WithCancel(ctx)
	errChan := make(chan error)
	shutdown := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/quitquitquit", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = rw.Write([]byte(`{"status":"error","message":"quitquitquit must be a POST request"}`))
			return
		}

		log.Println("Received quitquitquit request. Canceling context...")
		cancel()
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"status": "ok","message":"shutdown initiated"}`))
	})

	srv := &http.Server{Addr: ServerAddr, Handler: mux}

	go func() {
		err := generateIDToken(handleSignals(mainCtx), saInfo, token, file, refresh)
		errChan <- err
	}()

	go listenForPreemption(mainCtx, srv, shutdown)

	go srv.ListenAndServe()

	mainErr := <-errChan
	<-shutdown

	return mainErr
}

func generateIDTokenCmd(c *cli.Context) error {
	return startServerAndGenerator(c.Context, gcp.NewSaInfo(), gcp.NewIDToken(), c.String("file"), c.Bool("refresh"))
}

func listenForPreemption(ctx context.Context, srv *http.Server, shutdown chan struct{}) {
	<-ctx.Done()
	log.Println("server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.SetKeepAlivesEnabled(false)
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("could not gracefully shutdown the server: %v", err)
	}
	close(shutdown)
}

func handleSignals(baseContext context.Context) context.Context {
	// Graceful shut-down on SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// create cancelable context
	ctx, cancel := context.WithCancel(baseContext)

	go func() {
		defer cancel()
		select {
		case <-ctx.Done():
			log.Println("context canceled. exiting goroutine...")
		case sid := <-sig:
			log.Printf("received signal: %d\n", sid)
			log.Println("canceling token refresh ...")
		}
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
