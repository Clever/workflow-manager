#!/usr/bin/env bash

set -euo pipefail

while read -r w; do
    echo y | go run main.go -workflow "${w}:master" -status running -cmd refresh -rate-limit 100
    echo y | go run main.go -workflow "${w}:master" -status queued -cmd refresh -rate-limit 100
done < workflows.txt
