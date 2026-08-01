# fvtt-migrate

Inspects a Foundry Virtual Tabletop installation and reports what a migration
would move: worlds, systems, modules, and which assets are actually referenced.

The tool never writes to or deletes anything in the installation it scans.

## Usage

Three steps: look, decide, move.

    fvtt-migrate scan  --root <user data> [--core <application dir>]
    fvtt-migrate plan  --root <user data> [--check-updates] [--target-core 14.363]
    fvtt-migrate apply --root <user data> --to <directory>

`--root` is the directory holding `Config` and `Data`. `--core` points at the
Foundry application itself; without it, references to built-in assets are
recognised by prefix rather than by checking that the file exists.

`plan` writes a `plan.yaml` you are meant to read and edit before applying it.
Nothing leaves the machine unless `--check-updates` is given, and that only
reads package manifests.

Packages the target can fetch for itself are not transferred. Files are moved by
content, so duplicates travel once and an interrupted run resumes where it
stopped.

## Status

Early development, though `scan`, `plan`, `apply` and `verify` work end to end
against a real installation. The licence key and the admin password hash are
never read, copied or transferred, and a test asserts it.

## Licence

GNU GPL v3. The tool is meant to be run against your own campaign data, so you
are entitled to read what it does with it.

## Building

Requires Go 1.26 or newer.

    go build ./cmd/fvtt-migrate

Cross-compiling for Windows:

    GOOS=windows GOARCH=amd64 go build ./cmd/fvtt-migrate
