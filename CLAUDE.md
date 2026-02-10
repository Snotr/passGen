# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

passGen — a customisable password generator written in Go.

## Build & Test Commands

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go test ./... -v        # verbose test output
go test -run TestName ./path/to/pkg  # run a single test
go vet ./...            # static analysis
```

## Notes

- Module path: `github.com/Snotr/passGen`
- The project uses an MIT license.
