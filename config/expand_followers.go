package config

import (
	"context"

	"bsky.watch/utils/didset"
	"github.com/bluesky-social/indigo/xrpc"
)

type expandFollowers struct {
	client *xrpc.Client
	set    didset.DIDSet
}

func (s *expandFollowers) GetDIDs(ctx context.Context) (didset.StringSet, error) {
	l, err := s.set.GetDIDs(ctx)
	if err != nil {
		return nil, err
	}

	followers := []didset.DIDSet{}
	for did := range l {
		followers = append(followers, didset.FollowersOf(s.client, did))
	}
	return didset.Union(followers...).GetDIDs(ctx)
}
