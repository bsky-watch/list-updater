package config

import (
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"

	"bsky.watch/utils/didset"
)

type List struct {
	Name           string       `json:"name" yaml:"name"`
	DID            string       `json:"did" yaml:"did"`
	Rkey           string       `json:"rkey" yaml:"rkey"`
	Entries        *ListEntries `json:"entries" yaml:"entries"`
	NoAutoRemovals bool         `json:"noAutoRemovals" yaml:"noAutoRemovals"`
}

type ListEntries struct {
	// Only one of the fields is expected to be set at a time.

	Union           []ListEntries  `json:"union" yaml:"union"`
	Intersection    []ListEntries  `json:"intersection" yaml:"intersection"`
	Difference      *SetDifference `json:"difference" yaml:"difference"`
	List            *MuteList      `json:"list" yaml:"list"`
	Followers       *string        `json:"followers" yaml:"followers"`
	Follows         *string        `json:"follows" yaml:"follows"`
	BlockedBy       *string        `json:"blockedBy" yaml:"blockedBy"`
	DID             *string        `json:"did" yaml:"did"`
	File            *string        `json:"file" yaml:"file"`
	ExpandFollowers *ListEntries   `json:"expandFollowers" yaml:"expandFollowers"`
}

type MuteList struct {
	DID  string `json:"did" yaml:"did"`
	Rkey string `json:"rkey" yaml:"rkey"`
}

type SetDifference struct {
	Left  *ListEntries `json:"left" yaml:"left"`
	Right *ListEntries `json:"right" yaml:"right"`
}

type Config struct {
	Lists []List `json:"list"`
}

func (e *ListEntries) AsSet(client *xrpc.Client) didset.DIDSet {
	switch {
	case e.Union != nil:
		sets := []didset.DIDSet{}
		for _, sub := range e.Union {
			sets = append(sets, sub.AsSet(client))
		}
		return didset.Union(sets...)
	case e.Intersection != nil:
		sets := []didset.DIDSet{}
		for _, sub := range e.Intersection {
			sets = append(sets, sub.AsSet(client))
		}
		return didset.Intersection(sets...)
	case e.Difference != nil:
		return didset.Difference(e.Difference.Left.AsSet(client), e.Difference.Right.AsSet(client))
	case e.List != nil:
		return didset.MuteList(client, fmt.Sprintf("at://%s/app.bsky.graph.list/%s", e.List.DID, e.List.Rkey))
	case e.Followers != nil:
		return didset.FollowersOf(client, *e.Followers)
	case e.Follows != nil:
		return didset.FollowRecordsOf(client, *e.Follows)
	case e.BlockedBy != nil:
		return didset.BlockedBy(client, *e.BlockedBy)
	case e.DID != nil:
		return didset.Const(*e.DID)
	case e.File != nil:
		return didset.FromFile(*e.File)
	case e.ExpandFollowers != nil:
		return &expandFollowers{client: client, set: e.ExpandFollowers.AsSet(client)}
	}
	return nil
}
