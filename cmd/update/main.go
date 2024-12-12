package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"bsky.watch/utils/aturl"
	"bsky.watch/utils/xrpcauth"
	comatproto "github.com/bluesky-social/indigo/api/atproto"

	"bsky.watch/list-updater/config"
	"bsky.watch/list-updater/sync"
)

var (
	authFile       = flag.String("auth-file", "bsky.auth", "Path to the file with credentials")
	listUri        = flag.String("list", "", "List to update")
	addFromFile    = flag.String("add", "", "File with DIDs you want to add to the list")
	removeFromFile = flag.String("remove", "", "File with DIDs you want to remove from the list")
	configPath     = flag.String("config", "", "YAML config file describing the desired list state")
	dryRun         = flag.Bool("dry-run", false, "If set, will not actually make any changes")
)

func runMain(ctx context.Context) error {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	ctx = setupLogging(ctx)
	log := zerolog.Ctx(ctx)

	if *addFromFile == "" && *removeFromFile == "" && *configPath == "" {
		return fmt.Errorf("must provide at least one of --add, --remove, or --config")
	}
	if *configPath == "" && *listUri == "" {
		return fmt.Errorf("without --config, --list is required")
	}

	var cfg config.List

	if *configPath != "" {
		b, err := os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file %q: %s", *configPath, err)
		}

		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file: %s", err)
		}
	}

	if *listUri != "" {
		u, err := aturl.Parse(*listUri)
		if err != nil {
			return fmt.Errorf("parsing %q: %w", *listUri, err)
		}
		if u.Scheme != "at" {
			return fmt.Errorf("expected at:// URI")
		}
		parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
		if parts[0] != "app.bsky.graph.list" {
			return fmt.Errorf("expected a URI pointing to app.bsky.graph.list collection")
		}
		if len(parts) < 2 {
			return fmt.Errorf("missing rkey in the provided URI")
		}
		cfg.DID = u.Host
		cfg.Rkey = parts[1]
	}

	if cfg.Entries == nil {
		// Don't have a config or the config didn't specify entries.
		// Initialize to the current content of the list.
		cfg.Entries = &config.ListEntries{
			List: &config.MuteList{DID: cfg.DID, Rkey: cfg.Rkey},
		}

		if *addFromFile == "" && *removeFromFile == "" {
			return fmt.Errorf("no action specified, please either provide desired entries in the config file or use --add and/or --remove")
		}
	}

	if *addFromFile != "" {
		cfg.Entries = &config.ListEntries{
			Union: []config.ListEntries{
				*cfg.Entries,
				{File: addFromFile},
			},
		}
	}

	if *removeFromFile != "" {
		cfg.Entries = &config.ListEntries{
			Difference: &config.SetDifference{
				Left:  cfg.Entries,
				Right: &config.ListEntries{File: removeFromFile},
			},
		}
	}

	client := xrpcauth.NewClient(ctx, *authFile)
	session, err := comatproto.ServerGetSession(ctx, client)
	if err != nil {
		return fmt.Errorf("getting info about logged in session: %w", err)
	}

	if cfg.DID != session.Did {
		return fmt.Errorf("list belongs to a different user (%s) than our logged in session (%s)", cfg.DID, session.Did)
	}

	if *dryRun {
		log.Printf("Simulating the update of the list at://%s/app.bsky.graph.list/%s...", cfg.DID, cfg.Rkey)
	} else {
		log.Printf("Updating the list at://%s/app.bsky.graph.list/%s...", cfg.DID, cfg.Rkey)
	}
	return sync.UpdateMuteList(ctx, cfg.Entries.AsSet(client), fmt.Sprintf("at://%s/app.bsky.graph.list/%s", cfg.DID, cfg.Rkey), client, !cfg.NoAutoRemovals, *dryRun)
}

func main() {
	flag.Parse()

	if err := runMain(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupLogging(ctx context.Context) context.Context {
	var output io.Writer

	output = zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    true,
		TimeFormat: time.RFC3339,
		PartsOrder: []string{
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			zerolog.TimestampFieldName,
			zerolog.MessageFieldName,
		},
	}

	logger := zerolog.New(output).Level(zerolog.Level(zerolog.DebugLevel)).With().Caller().Timestamp().Logger()

	ctx = logger.WithContext(ctx)

	zerolog.DefaultContextLogger = &logger
	log.SetOutput(logger)

	return ctx
}
