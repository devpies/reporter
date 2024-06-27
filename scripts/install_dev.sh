#!/bin/bash

# GetGo retrieves and installs Go for Linux and macOS operating systems.
function GetGo() {
  # Check if Go is installed
  if ! command -v go &> /dev/null; then
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
      sudo apt update
      sudo apt install -y golang
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      # Check if Homebrew is installed
      if ! command -v brew &> /dev/null; then
        echo "Homebrew not found. Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        # Add Homebrew to the PATH
        echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
        eval "$(/opt/homebrew/bin/brew shellenv)"
      fi
    brew install go
    fi
  fi
}

# GetGolangCILint retrieves and installs Golang CI Linter for Linux and macOS operating systems.
function GetGolangCILint() {
  # Check if golangci-lint is installed
  if ! command -v golangci-lint &> /dev/null; then
    echo "golangci-lint not found. Installing golangci-lint..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
      # Install golangci-lint for Linux
      curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin 1.59.1
      # Add GOPATH/bin to the PATH
      export PATH=$PATH:$(go env GOPATH)/bin
      echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.profile
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      # Check if Homebrew is installed
      if ! command -v brew &> /dev/null; then
        echo "Homebrew not found. Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        # Add Homebrew to the PATH
        echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
        eval "$(/opt/homebrew/bin/brew shellenv)"
      fi
      brew install golangci-lint@1.59.1
    fi
  fi
}

GetGo
GetGolangCILint