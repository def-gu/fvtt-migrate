# fvtt-migrate

Inspects a Foundry Virtual Tabletop installation and reports what a migration
would move: worlds, systems, modules, and which assets are actually referenced.

The tool never writes to or deletes anything in the installation it scans.

## Status

Early development. Only `scan` is implemented.

## Usage

    fvtt-migrate scan --root /path/to/FoundryVTT/Data

## Building

Requires Go 1.26 or newer.

    go build ./cmd/fvtt-migrate

Cross-compiling for Windows:

    GOOS=windows GOARCH=amd64 go build ./cmd/fvtt-migrate
