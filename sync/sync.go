package sync

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/imax9000/errors"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"

	"bsky.watch/utils/aturl"
	"bsky.watch/utils/didset"
)

func splitInBatches[T any](s []T, batchSize int) [][]T {
	var r [][]T
	for i := 0; i < len(s); i += batchSize {
		if i+batchSize < len(s) {
			r = append(r, s[i:i+batchSize])
		} else {
			r = append(r, s[i:])
		}
	}
	return r
}

const maxThrottlingWait time.Duration = 5 * time.Minute

// UpdateMuteList adds and removes entries to/from the list to make it match the provided set.
//
// If removeMissing is false, no entries are going to be removed from the list.
func UpdateMuteList(ctx context.Context, set didset.DIDSet, listUrl string, client *xrpc.Client, removeMissing bool, dryRun bool) error {
	log := zerolog.Ctx(ctx)

	source, err := set.GetDIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the source set: %w", err)
	}

	log.Printf("Got %d accounts", len(source))

	resp, err := comatproto.ServerGetSession(ctx, client)
	if err != nil {
		return fmt.Errorf("bsky.ServerGetSession: %w", err)
	}
	self := resp.Did

	cursor := ""
	removed := 0
	toDelete := []string{}
	for {
	retry0:
		resp, err := comatproto.RepoListRecords(ctx, client, "app.bsky.graph.listitem", cursor, 100, self, false, "", "")
		if err != nil {
			if err, ok := errors.As[*xrpc.Error](err); ok {
				if err.IsThrottled() && err.Ratelimit != nil {
					log.Printf("Got throttled until %s", err.Ratelimit.Reset)
					if d := time.Until(err.Ratelimit.Reset); d <= maxThrottlingWait {
						time.Sleep(d)
						goto retry0
					}
				}
			}
			return fmt.Errorf("RepoListRecords: %w", err)
		}
		for _, r := range resp.Records {
			item, ok := r.Value.Val.(*bsky.GraphListitem)
			if !ok {
				continue
			}
			if item.List != listUrl {
				continue
			}
			if source[item.Subject] {
				delete(source, item.Subject)
				continue
			}

			if removeMissing {
				url, err := aturl.Parse(r.Uri)
				if err != nil {
					log.Printf("Failed to parse listitem URL %q: %s", r.Uri, err)
					continue
				}
				rkey := path.Base(url.Path)
				toDelete = append(toDelete, rkey)
				log.Printf("Added %q to removal queue", item.Subject)
			}
		}
		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	const batchSize = 50
	if !dryRun {
		for _, batch := range splitInBatches(toDelete, batchSize) {
			req := &comatproto.RepoApplyWrites_Input{
				Repo: self,
			}
			for _, rkey := range batch {
				req.Writes = append(req.Writes, &comatproto.RepoApplyWrites_Input_Writes_Elem{
					RepoApplyWrites_Delete: &comatproto.RepoApplyWrites_Delete{
						Collection: "app.bsky.graph.listitem",
						Rkey:       rkey,
					},
				})
			}

		retry1:
			if err := comatproto.RepoApplyWrites(ctx, client, req); err != nil {
				log.Printf("Failed to apply deletions to the list: %s", err)
				if err, ok := errors.As[*xrpc.Error](err); ok {
					if err.IsThrottled() && err.Ratelimit != nil {
						log.Printf("Got throttled until %s", err.Ratelimit.Reset)
						if d := time.Until(err.Ratelimit.Reset); d <= maxThrottlingWait {
							time.Sleep(d)
							goto retry1
						}
					}
				}
				log.Printf("applyWrites failed (rkeys: %v): %s", batch, err)
				continue
			}
			removed += len(batch)
		}
		log.Printf("Removed %d accounts. Have %d accounts left to add.", removed, len(source))
	}

	toAdd := maps.Keys(source)
	if !dryRun {
		for _, batch := range splitInBatches(toAdd, batchSize) {
			req := &comatproto.RepoApplyWrites_Input{
				Repo: self,
			}
			for _, did := range batch {
				req.Writes = append(req.Writes, &comatproto.RepoApplyWrites_Input_Writes_Elem{
					RepoApplyWrites_Create: &comatproto.RepoApplyWrites_Create{
						Collection: "app.bsky.graph.listitem",
						Value: &lexutil.LexiconTypeDecoder{Val: &bsky.GraphListitem{
							List:      listUrl,
							Subject:   did,
							CreatedAt: time.Now().UTC().Format(time.RFC3339),
						}},
					},
				})
			}

		retry2:
			if err := comatproto.RepoApplyWrites(ctx, client, req); err != nil {
				log.Printf("Failed to apply additions to the list: %s", err)
				if err, ok := errors.As[*xrpc.Error](err); ok {
					if err.IsThrottled() && err.Ratelimit != nil {
						log.Printf("Got throttled until %s", err.Ratelimit.Reset)
						if d := time.Until(err.Ratelimit.Reset); d <= maxThrottlingWait {
							time.Sleep(d)
							goto retry2
						}
					}
				}
				return err
			}

			for _, did := range batch {
				log.Printf("Added %q", did)
			}
		}
	} else {
		// dryRun
		for did := range toAdd {
			log.Printf("Would add %q", did)
		}
	}

	if !dryRun && removed < len(toDelete) {
		return fmt.Errorf("some deletions failed. See previous log entries")
	}
	if dryRun {
		log.Printf("Would add %d entries and remove %d entries.", len(toAdd), len(toDelete))
	}

	return nil
}
