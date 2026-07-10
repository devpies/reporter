#!/bin/bash
set -euo pipefail

# INSTALL DEV
#
# This script prepares a developer environment by installing the Go toolchain
# and the golangci-lint binary.
#
# It is intentionally designed to install the *latest available* versions every
# time it is executed:
#   - On Linux, Go is installed via apt and golangci-lint via `go install`.
#   - On macOS, Go and golangci-lint are installed via Homebrew.
#
# If the latest version is already present, the package managers will perform
# no action (Linux apt / Homebrew). On Linux, golangci-lint will always be
# reinstalled to ensure it matches the latest release.
#
# NOTE: golangci-lint's official curl-piped install.sh script was previously used
# on Linux, but its release-asset checksum verification has been unreliable
# (deterministic checksum mismatches against the published release tarball).
# `go install` is used instead since it verifies modules via the Go checksum
# database (GOSUMDB) rather than a hand-rolled release checksum file.

function install_go() {
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    sudo apt update
    sudo apt install -y golang   # latest available in apt repo
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    if ! command -v brew &> /dev/null; then
      echo "Homebrew not found. Installing Homebrew..."
      /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
      echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
      eval "$(/opt/homebrew/bin/brew shellenv)"
    fi
    brew update
    brew install go
  fi
}

function install_golangci_lint() {
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    GOBIN="$(go env GOPATH)/bin"
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    export PATH="$PATH:$GOBIN"
    if ! grep -q "$GOBIN" ~/.profile 2>/dev/null; then
      echo "export PATH=\"\$PATH:$GOBIN\"" >> ~/.profile
    fi
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    if ! command -v brew &> /dev/null; then
      echo "Homebrew not found. Installing Homebrew..."
      /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
      echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
      eval "$(/opt/homebrew/bin/brew shellenv)"
    fi
    brew update
    brew install golangci-lint
  fi
}

install_go
install_golangci_lint