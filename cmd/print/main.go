package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"bsky.watch/utils/xrpcauth"

	"bsky.watch/list-updater/config"
)

var (
	authFile   = flag.String("auth-file", "bsky.auth", "Path to the file with credentials")
	configPath = flag.String("config", "", "YAML config file describing the desired list state")
)

func runMain(ctx context.Context) error {
	var cfg config.List

	b, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %q: %s", *configPath, err)
	}

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %s", err)
	}

	client := xrpcauth.NewClient(ctx, *authFile)

	set := cfg.Entries.AsSet(client)

	entries, err := set.GetDIDs(ctx)
	if err != nil {
		return err
	}
	for did := range entries {
		fmt.Println(did)
	}

	return nil
}

func main() {
	flag.Parse()

	if err := runMain(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
