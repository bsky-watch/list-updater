package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"bsky.watch/utils/xrpcauth"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"

	"bsky.watch/list-updater/config"
	"bsky.watch/list-updater/sync"
)

var (
	configPath = flag.String("config", "", "Path to the config file")
)

func main() {
	flag.Parse()
	ctx := setupLogging(context.Background())
	log := zerolog.Ctx(ctx)

	if *configPath == "" {
		log.Fatal().Msgf("Need --config")
	}

	var anonClient *xrpc.Client

	didToClient := map[string]*xrpc.Client{}

	if secrets := os.Getenv("GITHUB_SECRETS_JSON"); secrets != "" {
		fmt.Fprintf(os.Stderr, "::group::Logging in\n")
		vars := map[string]string{}
		if err := json.Unmarshal([]byte(secrets), &vars); err != nil {
			log.Fatal().Err(err).Msgf("Failed to unmarshal GitHub secrets")
		}
		for name, value := range vars {
			if !strings.Contains(value, ":") {
				continue
			}
			parts := strings.SplitN(value, ":", 2)
			handle, password := parts[0], parts[1]

			client := xrpcauth.NewClientWithTokenSource(ctx, xrpcauth.PasswordAuth(handle, password))
			resp, err := comatproto.ServerGetSession(ctx, client)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to create sessions for %q", name)
				continue
			}

			if name == "DEFAULT" {
				log.Info().Msgf("%s: logged in successfully", name)
			} else {
				log.Info().Msgf("%s: logged in as %s", name, resp.Handle)
			}

			didToClient[resp.Did] = client

			if name == "DEFAULT" {
				anonClient = didToClient[resp.Did]
			}
		}
		fmt.Fprintf(os.Stderr, "::endgroup::\n")
	}

	if anonClient == nil {
		log.Fatal().Msgf("Missing a default client, which is required to fetch follower lists")
	}

	config := &config.Config{}
	b, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to read config file %q: %s", *configPath, err)
	}

	if err := yaml.Unmarshal(b, config); err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse config file: %s", err)
	}

	errors := 0

	os.Chdir(filepath.Dir(*configPath))

	for _, cfg := range config.Lists {
		if cfg.Entries == nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "::group::%s\n", cfg.Name)
		log := log.With().Str("config", cfg.Name).Logger()

		if err := applyConfig(ctx, &cfg, anonClient, didToClient); err != nil {
			errors++
			log.Error().Err(err).Msgf("Failed to update the list")
		}

		fmt.Fprintf(os.Stderr, "::endgroup::\n")
	}

	if errors > 0 {
		os.Exit(1)
	}
}

func applyConfig(ctx context.Context, cfg *config.List, defaultClient *xrpc.Client, didToClient map[string]*xrpc.Client) error {
	log := zerolog.Ctx(ctx).With().Str("config", cfg.Name).Logger()
	client := didToClient[cfg.DID]
	if client == nil {
		return fmt.Errorf("missing a client authenticated as %q", cfg.DID)
	}
	set := cfg.Entries.AsSet(defaultClient)

	return sync.UpdateMuteList(log.WithContext(ctx), set, ListURI(cfg.DID, cfg.Rkey), client, !cfg.NoAutoRemovals, false)
}
