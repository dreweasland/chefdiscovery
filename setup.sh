#!/bin/bash
set -euo pipefail

# Ensure we’re in the project directory.
cd "$(dirname "$0")"

# Initialize module if it doesn’t exist.
if [ ! -f go.mod ]; then
  go mod init example.com/chefdiscovery
fi

# Resolve and download all dependencies.
go mod tidy
go mod download
