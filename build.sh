#!/usr/bin/env bash

# Exit on error
set -e

# Create dist directory
mkdir -p dist

# Build the plugin
echo "Building kubectl-crdlist..."
go build -o dist/kubectl-crdlist .

echo "Build complete. Binary is at dist/kubectl-crdlist"
