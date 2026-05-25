# crap4x

Polyglot CRAP metric analyzer for Go, Python, and Rust.

**CRAP(f) = cc(f)² · (1 − cov(f))³ + cc(f)**

where `cc` is cyclomatic complexity and `cov` is test coverage (0–1) for a function.

## Usage

```
crap4x [path] --coverage cover.lcov [--threshold 30] [--top 20]
```

Full usage docs coming in v0.1.0.

## Attribution

The CRAP metric was introduced by Alberto Savoia and Bob Evans in [*Crap4J*](http://www.crap4j.org/).
Robert C. Martin subsequently published implementations for multiple languages —
`crap4go`, `crap4java`, and `crap4clj` — which inspired this project.

crap4x is an independent reimplementation that generalises the metric to Python, Go, and Rust.
It shares no code with the above projects and is released separately under the MIT License.

## License

MIT — Copyright (c) 2026 Andrew Naoya McWilliam
