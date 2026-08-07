# Tooling Reference

## Purpose

- Record durable repo-wide tooling notes, command references, and local development expectations
- Keep short-lived implementation notes in feature docs instead of here

## Current State

- `Makefile` is the canonical project command interface; `make` with no target prints `make help`
- `make check` wraps `kit check --project` to validate the project document and instruction contract
- Override the Kit binary with `make check KIT=/path/to/kit`
- No build, test, lint, or formatting commands exist yet; add targets only as real toolchain commands land
