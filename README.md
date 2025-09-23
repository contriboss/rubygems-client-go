# rubygems-client-go

> Go client library for the RubyGems.org API - a provider implementation for ORE

[![CI](https://github.com/contriboss/rubygems-client-go/actions/workflows/ci.yml/badge.svg)](https://github.com/contriboss/rubygems-client-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/contriboss/rubygems-client-go.svg)](https://pkg.go.dev/github.com/contriboss/rubygems-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/contriboss/rubygems-client-go)](https://goreportcard.com/report/github.com/contriboss/rubygems-client-go)

## Overview

This is a Go client library for the RubyGems.org API, designed as a provider implementation for [ORE](https://github.com/contriboss/ore), the fast Ruby gem installer. It's part of ORE's plugin architecture that allows swapping gem sources.

Ruby equivalent: `Gem::RemoteFetcher` and `Gem::SpecFetcher`

## Why This Exists

ORE uses a provider-based architecture to avoid vendor lock-in with RubyGems.org. This client is the default provider, but can be replaced with:
- Private gem servers
- GitHub Packages
- GitLab Package Registry
- Local caches
- Alternative gem repositories

## Quick Start

```bash
go get github.com/contriboss/rubygems-client-go
```

## Usage

### Basic API Client

```go
package main

import (
    "fmt"
    "log"

    rubygems "github.com/contriboss/rubygems-client-go"
)

func main() {
    client := rubygems.NewClient()

    // Get gem information
    info, err := client.GetGemInfo("rails", "7.0.0")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Gem: %s v%s\n", info.Name, info.Version)
    fmt.Printf("Runtime dependencies: %d\n", len(info.Dependencies.Runtime))

    // Get available versions
    versions, err := client.GetGemVersions("rails")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Available versions: %v\n", versions[:5]) // Show first 5
}
```

### Parallel Fetching

```go
// Fetch multiple gems in parallel
requests := []rubygems.GemInfoRequest{
    {Name: "rails", Version: "7.0.0"},
    {Name: "puma", Version: "6.0.0"},
    {Name: "sidekiq", Version: "7.0.0"},
}

results := client.GetMultipleGemInfo(requests)

for _, result := range results {
    if result.Error != nil {
        log.Printf("Failed to fetch %s: %v", result.Request.Name, result.Error)
        continue
    }

    fmt.Printf("%s v%s has %d dependencies\n",
        result.Info.Name,
        result.Info.Version,
        len(result.Info.Dependencies.Runtime))
}
```

## Provider Interface

This client implements the ORE provider interface, allowing it to be used as a gem source:

```go
type Provider interface {
    GetGemInfo(name, version string) (*GemInfo, error)
    GetVersions(name string) ([]string, error)
    GetDependencies(name, version string) ([]Dependency, error)
    // Additional methods for downloading gems
}
```

## Features

- **Connection pooling** for efficient HTTP requests
- **Parallel fetching** for multiple gems
- **Automatic retries** with exponential backoff
- **Version limiting** to avoid overwhelming resolvers
- **Ruby engine compatibility**

## Architecture

This is one component of ORE's modular architecture:

```
ORE (orchestrator)
    ↓
Provider Interface
    ↓
rubygems-client-go (this library)
    ↓
RubyGems.org API
```

## Building

We use [Mage](https://magefile.org) for builds:

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Run tests
mage test

# Run linter
mage lint

# Build
mage build

# Run CI checks
mage ci
```

## Testing

```bash
# Run tests with coverage
mage test

# Run tests with race detector
mage testrace

# Run benchmarks
mage bench
```

## Performance

- Parallel gem fetching with configurable worker pool
- Connection reuse reduces latency
- Automatic retry on transient failures
- Efficient caching in ORE layer

## License

MIT

## Related Projects

- [ORE](https://github.com/contriboss/ore) - The fast Ruby gem installer
- [gemfile-go](https://github.com/contriboss/gemfile-go) - Parse Gemfile and Gemfile.lock
- [ruby-extension-go](https://github.com/contriboss/ruby-extension-go) - Build Ruby native extensions

---

Made by [@contriboss](https://github.com/contriboss) - Part of the ORE ecosystem