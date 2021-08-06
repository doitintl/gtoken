package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/doitintl/gtoken/internal/aws"
	"github.com/doitintl/gtoken/internal/gcp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

var (
	// Version contains the current Version.
	Version = "dev"
	// BuildDate contains a string with the build BuildDate.
	BuildDate = "unknown"
	// GitCommit git commit sha
	GitCommit = "dirty"
	// GitBranch git branch
	GitBranch = "dirty"
	// Platform OS/ARCH
	Platform = ""
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "file",
				Usage: "filepath to write ID token to",
			},
			&cli.BoolFlag{
				Name:  "refresh",
				Value: false,
				Usage: "auto refresh ID token before it expires",
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "copy",
				Aliases:   []string{"cp"},
				Usage:     "copy itself to a destination folder",
				ArgsUsage: "destination",
				Action:    copyCmd,
			},
		},
		Name:    "gtoken-daemon",
		Usage:   "generate ID token with current Google Cloud service account, within the main app container",
		Action:  mainCmd,
		Version: Version,
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version: %s\n", Version)
		fmt.Printf("  build date: %s\n", BuildDate)
		fmt.Printf("  commit: %s\n", GitCommit)
		fmt.Printf("  branch: %s\n", GitBranch)
		fmt.Printf("  platform: %s\n", Platform)
		fmt.Printf("  built with: %s\n", runtime.Version())
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func copyCmd(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("must specify copy destination")
	}
	// full path of current executable
	src := os.Args[0]
	// destination path
	dest := filepath.Join(c.Args().First(), filepath.Base(src))
	// copy file with current file mode flags
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	srcInfo, err := source.Stat()
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	destination, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()
	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}
	return destination.Chmod(srcInfo.Mode())
}

func mainCmd(c *cli.Context) error {
	// Routine to reap zombies (it's the job of init)
	ctx, cancel := context.WithCancel(context.Background())

	var mainRC int
	var wg sync.WaitGroup
	wg.Add(1)

	go removeZombies(ctx, &wg)

	ok, index := sliceContains(os.Args, "--")
	if !ok {
		log.Error("app invocation and arguments should be prefixed by -- , e.g. gtoken-daemon --daemon -- myapp foo")
	}
	command := os.Args[index:]
	binary, err := exec.LookPath(command[0])
	if err != nil {
		log.WithError(err).Errorf("could not find application executable: %s", command[0])
		mainRC = -2
	} else {
		// Launch main command
		mainRC, err = run(ctx, &wg, binary, c, command[1:])
		if err != nil {
			log.WithError(err).Error("failed to run")
		}
	}
	// Cancel all remaining goroutines gracefully before returning the main command OS exit code
	cleanQuit(cancel, &wg, mainRC)
	return nil
}

func removeZombies(ctx context.Context, wg *sync.WaitGroup) {
	for {
		var status syscall.WaitStatus

		// wait for an orphaned zombie process
		pid, _ := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)

		if pid <= 0 {
			// PID is 0 or -1 if no child waiting, so we wait for 1 second for next check
			time.Sleep(1 * time.Second)
		} else {
			// PID is > 0 if a child was reaped and we immediately check if another one is waiting
			continue
		}

		// non-blocking test if context is done
		select {
		case <-ctx.Done():
			// context is done, so we stop goroutine
			wg.Done()
			return
		default:
		}
	}
}

// run passed command
func run(ctx context.Context, wg *sync.WaitGroup, binary string, c *cli.Context, args []string) (int, error) {
	// register a channel to receive system signals
	sigs := make(chan os.Signal, 1)
	defer close(sigs)
	signal.Notify(sigs)
	defer signal.Reset()

	// define a command and rebind its stdout and stdin
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set AWS environment variables needed by the AWS SDK for authentication
	awsEnvVariables, err := aws.NewAWSEnvVariables().Generate()
	if err != nil {
		log.WithError(err)
		return -2, err
	}
	cmd.Env = append(os.Environ(), awsEnvVariables...)

	// create a dedicated pidgroup used to forward signals to the main process and its children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Handle GCP OIDC token generation
	tokenFileDone := make(chan bool, 1)
	defer close(tokenFileDone)
	wg.Add(1)
	go handleTokenID(ctx, wg, c, tokenFileDone)

	// start the specified command
	log.WithFields(log.Fields{
		"command": binary,
		"args":    args,
		"env":     cmd.Env,
	}).Debug("starting command")
	err = cmd.Start()
	if err != nil {
		log.WithError(err).Error("failed to start command")
		return -2, err
	}

	// Goroutine for signals forwarding
	wg.Add(1)
	go handleSignals(ctx, wg, sigs, cmd)

	// wait for the command to exit
	err = cmd.Wait()
	exitCode := cmd.ProcessState.ExitCode()
	if err != nil {
		log.WithError(err).Error("failed to wait for command to complete")
		return exitCode, err
	}
	return exitCode, err
}

// handleSignals handles signal forwarding to the forked process
func handleSignals(ctx context.Context, wg *sync.WaitGroup, sigs chan os.Signal, cmd *exec.Cmd) {
	select {
	case <-ctx.Done():
		wg.Done()
		return
	case <-sigs:
		for sig := range sigs {
			// TODO: ask Alexei!
			// ignore SIGCHLD signals since these are only useful for secrets-init
			if sig != syscall.SIGCHLD {
				// forward signal to the main process and its children
				e := syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
				if e != nil {
					log.WithFields(log.Fields{
						"pid":    cmd.Process.Pid,
						"path":   cmd.Path,
						"args":   cmd.Args,
						"signal": unix.SignalName(sig.(syscall.Signal)),
					}).WithError(e).Error("failed to send system signal to the process")
				}
			}
		}
	}
}

// handleTokenID handles everything related to the GCP token generation and refresh
func handleTokenID(ctx context.Context, wg *sync.WaitGroup, c *cli.Context, tokenFileDone chan bool) {
	sa := gcp.NewSaInfo()
	idToken := gcp.NewIDToken()

	// initial duration to 1ms
	duration := time.Millisecond
	timer := time.NewTimer(duration).C

	// find out active Service Account, first by ID
	serviceAccount, err := sa.GetID(ctx)
	if err != nil {
		log.WithError(err).Error("failed to get service account, fallback to metadata email")
		// fallback: try to get Service Account email from metadata server
		serviceAccount, err = sa.GetEmail()
	}
	if err != nil {
		log.WithError(err).Error("failed to get service account from metadata email")
	}

	for {
		// wait for next timer tick or cancel
		select {
		case <-ctx.Done():
			wg.Done()
			return
		case <-timer:
			// generate ID token
			token, err := idToken.Generate(ctx, serviceAccount)
			if err != nil {
				log.WithError(err).Error("failed to generate token")
				return
			}
			// write generated token to file or stdout
			err = idToken.WriteToFile(token, c.String("file"))
			if err != nil {
				log.WithError(err).Error("failed to write token to file")
				return
			}

			// TODO: check if channel is already closed to avoid sending to a closed channel
			tokenFileDone <- true

			// auto-refresh enabled
			if c.Bool("refresh") {
				// get token duration
				duration, err = idToken.GetDuration(token)
				if err != nil {
					log.WithError(err).Error("failed to get token duration")
				}
				// reduce duration by 30s
				duration -= 30 * time.Second
				log.Printf("refreshing token in %s", duration)
				// reset timer
				timer = time.NewTimer(duration).C
			} else {
				return // avoid goroutine leak
			}
		}
	}
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

func cleanQuit(cancel context.CancelFunc, wg *sync.WaitGroup, code int) {
	// signal all goroutines to stop and wait for it to release the wait group
	cancel()
	wg.Wait()
	os.Exit(code)
}
