#!/bin/sh

handle="$1"
outfile="${2:-bsky.auth}"
read -p "Password: " password

curl -s --data '{"identifier": "'"${handle}"'", "password": "'"${password}"'"}' \
	-H 'Content-Type: application/json' \
  https://bsky.social/xrpc/com.atproto.server.createSession > "${outfile}"
