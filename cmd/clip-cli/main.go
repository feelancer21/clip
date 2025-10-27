// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/urfave/cli/v2"
)

var (
	// version is set via ldflags at build time
	version = "dev"
)

func withApp(fn func(app *ClipApp) error) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		app, err := NewApp(c)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := app.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "error closing app: %v\n", closeErr)
			}
		}()

		return fn(app)
	}
}

func getInfo(app *ClipApp) error {
	return app.GetInfo()
}

func ListNodeAnnouncements(app *ClipApp) error {
	return app.ListNodeAnnouncements()
}

func listNodeInfo(app *ClipApp) error {
	return app.ListNodeInfo()
}

func publishNodeAnnouncement(app *ClipApp) error {
	return app.PublishNodeAnnouncement()
}

func publishNodeInfo(app *ClipApp) error {
	return app.PublishNodeInfo()
}

func generateKey(c *cli.Context) error {
	var (
		filename string
		err      error
	)

	if c.IsSet("keyfile") {
		filename = c.String("keyfile")
	} else if filename, err = defaultKeyPath(); err != nil {
		return err
	}

	nsec, err := nip19.EncodePrivateKey(nostr.GeneratePrivateKey())
	if err != nil {
		return fmt.Errorf("encoding private key: %w", err)
	}

	err = saveNsec(filename, nsec)
	if err != nil {
		return fmt.Errorf("saving private key: %w", err)
	}

	fmt.Printf("Generated new private key and saved to %s\n", filename)
	return nil
}

func run() int {
	// main ctx that cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sinceFlag := &cli.DurationFlag{Name: "since", Usage: "only fetch events published since the given duration ago (e.g., 24h, 30m).", Value: time.Hour * 24 * 60}
	timeoutFlag := &cli.DurationFlag{Name: "timeout", Usage: "maximum time to wait for fetching events.", Value: time.Second * 120}
	pubkeyFlag := &cli.StringFlag{Name: "pubkey", Usage: "Lightning node public key to filter events by."}
	showErrorsFlag := &cli.BoolFlag{Name: "show-errors", Usage: "show fetch errors alongside results.", Value: false}

	app := &cli.App{
		Name:    "clip-cli",
		Version: version,
		Usage: "CLIP (Common Lightning-node Information Payloader) - Sending " +
			"and receiving verifiable Lightning node information over Nostr.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "name of the config file (default ~/.config/clip/config.yaml)"},
		},
		Commands: []*cli.Command{
			{
				Name:   "getinfo",
				Usage:  "Returns basic information about the connected Lightning node.",
				Action: withApp(getInfo),
			},
			{
				Name:  "generatekey",
				Usage: "Generates a new private key for Nostr.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "keyfile", Usage: "name of the key file (default ~/.config/clip/key)."},
				},
				Action: generateKey,
			},
			{
				Name:    "listnodeannouncements",
				Aliases: []string{"lna"},
				Usage:   "Fetches all node announcement events from the configured Nostr relays and displays them.",
				Action:  withApp(ListNodeAnnouncements),
				Flags: []cli.Flag{
					sinceFlag,
					timeoutFlag,
					pubkeyFlag,
					showErrorsFlag,
				},
			},
			{
				Name:    "listnodeinfo",
				Aliases: []string{"lni"},
				Usage:   "Fetches all node information from the configured Nostr relays and displays it.",
				Action:  withApp(listNodeInfo),
				Flags: []cli.Flag{
					sinceFlag,
					timeoutFlag,
					pubkeyFlag,
					showErrorsFlag,
				},
			},
			{
				Name:    "pubnodeannounce",
				Aliases: []string{"pna"},
				Usage:   "Publishes a node announcement event to the configured Nostr relays.",
				Action:  withApp(publishNodeAnnouncement),
			},
			{
				Name:    "pubnodeinfo",
				Aliases: []string{"pni"},
				Usage:   "Publishes the node information specified in the config to the configured Nostr relays.",
				Action:  withApp(publishNodeInfo),
			},
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
