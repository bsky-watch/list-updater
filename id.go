package main

import "fmt"

func ListURI(did string, rkey string) string {
	return fmt.Sprintf("at://%s/app.bsky.graph.list/%s", did, rkey)
}
