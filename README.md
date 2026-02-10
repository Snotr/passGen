# passGen

A high-performance, interactive CLI password generator written in Go. It reads tagged data from `.txt` and `.csv` files and produces combinatorial passwords using concurrent pipelines.

## Installation

```bash
go install github.com/Snotr/passGen/cmd/passgen@latest
```

Or build from source:

```bash
git clone https://github.com/Snotr/passGen.git
cd passGen
go build -o passgen ./cmd/passgen/
```

## Quick Start

```bash
# Generate all combinations from a tagged word list
passgen words.txt

# Use a specific pattern and limit output
passgen -pattern "{names}{numbers}{symbols}" -amount 100 words.txt

# Multiple input files, CSV output to file
passgen -input words.txt,names.csv -format csv -output passwords.csv

# Time-limited generation
passgen -time 30s words.txt

# Case-expanded passwords with deduplication
passgen -case -unique words.txt

# All group subsets (single + pairs + full)
passgen -flex -unique words.txt
```

## Input File Formats

### TXT (tag-based)

Lines starting with `#tag` define groups. All subsequent lines belong to that group until a new tag appears.

```
#names
alice
bob
#numbers
42
99
#symbols
!
@
```

Lines before any explicit tag are auto-classified:
- All digits -> `#numbers`
- All punctuation/symbols -> `#symbols`
- Otherwise -> `#words`

### CSV (column-based)

Column headers define tag groups. Each column's data belongs to that tag.

```csv
#names,#numbers,#symbols
alice,1,!
bob,2,@
```

## Generation Modes

### Default mode (all permutations)

With no `-pattern` flag, passGen generates every combination of one item from each group in every possible group ordering.

For groups `#a(2 values)` and `#b(3 values)`: total = `2! x 2 x 3 = 12` combinations.

### Pattern mode

Use `-pattern` to specify a fixed ordering of groups:

```bash
passgen -pattern "{names}{numbers}{symbols}" input.txt
```

This produces only the Cartesian product in the specified order (no group-order permutation).

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-input` | | Comma-separated input file paths (`.txt`, `.csv`) |
| `-output` | stdout | Output file path |
| `-format` | `txt` | Output format: `txt` or `csv` |
| `-pattern` | | Generation template, e.g. `{names}{numbers}{symbols}` |
| `-amount` | `0` | Max passwords to generate (`0` = unlimited) |
| `-time` | `0` | Max duration, e.g. `30s`, `5m` (`0` = unlimited) |
| `-quiet` | `false` | Suppress progress output |
| `-case` | `false` | Expand values to UPPER, Camel, and lower variants |
| `-unique` | `false` | Eliminate duplicate passwords |
| `-flex` | `false` | Generate from every non-empty subset of groups (default mode only) |

Input files can also be passed as positional arguments:

```bash
passgen -amount 100 words.txt names.csv
```

## Advanced Features

### Case expansion

With `-case`, every value in each group is tripled into UPPER, Camel, and lower variants before combining. This dramatically increases coverage of case permutations:

```bash
# Expand all values to three case variants
passgen -case words.txt

# With a pattern
passgen -case -pattern "{names}{numbers}" words.txt
```

For example, the value `alice` becomes `ALICE`, `Alice`, and `alice`. Values unaffected by case (like `123`) are automatically deduplicated to a single entry.

### Deduplication

Eliminate duplicate passwords across all orderings with `-unique`:

```bash
passgen -unique words.txt

# Case expansion + dedup to remove case-overlap duplicates
passgen -case -unique words.txt
```

### Flexible subset generation

With `-flex`, passGen generates passwords from every non-empty subset of groups (not just the full set). This is useful when you want single-group, two-group, and full-group combinations all at once:

```bash
passgen -flex words.txt

# Combine with dedup to remove overlaps
passgen -flex -unique words.txt
```

Note: `-flex` is ignored when `-pattern` is specified.

## Interactive Controls

When running in a terminal (not piped), passGen provides real-time controls:

- **`[p]`** - Pause / resume generation
- **`[s]`** - Stop generation (prompts to save partial results when writing to a file)
- **`Ctrl+C`** - Abort immediately

Progress is displayed on stderr so stdout remains clean for piping.

## Project Structure

```
cmd/passgen/          CLI entry point
internal/
  parser/             TXT and CSV file parsing with #tag grouping
  generator/          Concurrent combinatorial generation engine
  writer/             Buffered TXT and CSV output writers
  tui/                Interactive terminal controls and progress display
testdata/             Test fixtures
```

## Development

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go test -v -race ./...  # verbose with race detector
go vet ./...            # static analysis
gofmt -l .              # check formatting
```

## License

MIT
