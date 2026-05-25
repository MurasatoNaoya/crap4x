# crap4x

Polyglot CRAP metric analyzer for **Go, Python, and Rust**.

**CRAP(f) = cc(f)² · (1 - cov(f))³ + cc(f)**

where `cc` is the function's cyclomatic complexity and `cov` is its test coverage (0 to 1). High complexity combined with low coverage produces a high score, flagging the code most expensive and risky to change. crap4x parses each source file with tree-sitter to compute complexity, reads an lcov coverage report, joins them per function, and prints a ranked table.

## Install

```
go install github.com/MurasatoNaoya/crap4x/cmd/crap4x@latest
```

Or build from a clone: `go build ./cmd/crap4x`.

## Usage

```
crap4x [path] --coverage cover.lcov [--lang go,python,rust] [--threshold 30] [--top 20]
```

- `path` defaults to `.`. crap4x detects languages from project markers (`go.mod`, `Cargo.toml`, `pyproject.toml`/`setup.py`/`requirements.txt`); override with `--lang`.
- `--coverage` is an lcov file (see below). Without it, every function is scored at 0 coverage.
- `--threshold N` flags functions with CRAP above `N` and exits non-zero, for use in CI.
- `--top N` limits the table to the worst `N` functions.

## Producing an lcov report

| Language | Command |
|----------|---------|
| Go | `go test ./... -coverprofile=cover.out` then `gcov2lcov -infile=cover.out -outfile=cover.lcov` |
| Python | `coverage run -m pytest && coverage lcov -o cover.lcov` |
| Rust | `cargo llvm-cov --lcov --output-path cover.lcov` |

## Example

```
| Function   | File         | CC | Cov%  | CRAP  |
| ---------- | ------------ | -- | ----- | ----- |
| handleScan | scanner.go   | 13 | 42.0  | 168.4 |
| parseEntry | scanner.go   |  6 | 80.0  |   6.3 |
| simple     | util.go      |  1 | 100.0 |   1.0 |
```

A function with full coverage scores its complexity (the `(1 - cov)³` term vanishes); a complex, untested function scores `cc² + cc`.

## How complexity is computed

One tree-sitter pass per file. A function's complexity is `1 + ` the number of decision points in its body: `if`/`elif`, `for`, `while`, `switch`/`match` arms, `except`, the `&&`/`||`/`and`/`or` operators, ternary and `?` expressions. Decision points inside a nested function or closure count toward that inner function, not the outer one. An unconditional `loop` (Rust) adds nothing.

## Supported languages

Go, Python, Rust. Adding a language is a new `LangSpec` (its tree-sitter grammar plus the node types that count as functions and decisions); the analyzer and reporter are language-agnostic.

## Attribution

The CRAP metric was introduced by Alberto Savoia and Bob Evans in [*Crap4J*](http://www.crap4j.org/). Robert C. Martin later published implementations for several languages (`crap4go`, `crap4java`, `crap4clj`), which inspired this project.

crap4x is an independent reimplementation that generalises the metric to Python, Go, and Rust. It shares no code with those projects and is released separately under the MIT License.

## License

MIT. Copyright (c) 2026 Andrew Naoya McWilliam
