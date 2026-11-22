---
title: A test's expected value must not come from the code under test
date: 2026-11-21
category: best-practices
module: internal/judge
problem_type: best_practice
component: testing_framework
severity: medium
applies_when:
  - A test builds its expected output with the same encoder, table or shape the code under test uses
  - A test checks conformance to a specification that exists as a file or a table in the repository
  - A test asserts the absence of a spelling or value that only some request types ever write
tags: [testing, test-oracle, fixtures, encoder, schema, specification]
---

# A test's expected value must not come from the code under test

## Context

The judge's tests compare whole output documents byte for byte, which is strict. But in three places the expected side was produced by the code being tested, so a bug appeared on both sides of the comparison and cancelled out. Each was found by building a deliberate mistake into the source and seeing every test stay green:

- **The writer.** `assertBytes` in `internal/judge/fixtures_test.go` compares the judge's output with `write(want)`, where `write` (`internal/judge/json.go`) is the judge's own encoder. A change that made the encoder write every number as spelled, so a `-0` from the input reached the output, passed the whole suite. The test written to catch it, `TestIntegerOutputSpelling`, put the `-0` into a grade request's baseline, whose counts a grade never writes, so its "no `-0` in the output" assertion held whatever the encoder did.
- **The schema.** The schema test walked the judge's own hand-written input shape, so a mistake in that shape (for example, accepting a string where the schema requires an object) could not fail it.
- **The rulebook table.** The fixtures built the expected list of dimensions from the judge's own `dimensions` variable, so a misspelt or reordered dimension changed both sides at once.

## Guidance

Derive the expected value from the specification, not from the implementation:

- **Schemas:** read the schema file in the test. `TestEverySchemaBoundary` in `internal/judge/schema_test.go` now parses `docs/specs/judge-1.0/schema/judge-input.schema.json`, derives each location's type, nullability, enums, `minItems` and integer bounds from it, and checks the validator against that.
- **Tables from the rulebook:** hard-code them in the test file. `fixtureDimensions` in `internal/judge/fixtures_test.go` lists the 21 dimensions as literals.
- **Encoders:** check the bytes with literals the encoder did not produce. `TestWriterEscapes` compares the writer's output with a literal string, and `TestReviewOutputEscapes` looks for literal escape sequences with `bytes.Contains`. Any test that needs a property of the output's spelling, such as the absence of `-0`, must assert on the raw bytes as well as through `assertBytes`.
- **Absence assertions:** first make sure the request actually writes the field. `TestIntegerOutputSpelling` now uses an `anchor` request that carries the baseline entry into the output, injects `-0` into the raw input bytes (`write` would have turned it into `0`), and asserts both that `"k":0` is present and that `-0` is absent.

`assertBytes` itself still builds its expectation with `write`. That is fine for checking what the judge decides, because a wrong decision changes the document. It is not evidence about how the document is spelled. Encoder behaviour needs its own literal assertions.

## Why This Matters

A comparison is only as independent as its two sides. When the expected side is computed by the code under test, the test checks that the code agrees with itself, and every mistake shared by both sides passes. Neither the compiler, the lints nor the published cases noticed these three: only deliberate mutations did.

## When to Apply

- Writing a fixture helper that serialises the expected value with the production encoder.
- Testing conformance to anything the repository holds as a file or table (a JSON schema, a list of dimensions, rule ids, exit codes).
- Writing a test that a value or spelling must not appear: check that the request writes that field at all.

## Examples

Before, the expected dimensions came from the implementation:

```go
for _, d := range dimensions { // the judge's own table
	want = append(want, object{"dimension": d, ...})
}
```

After, they come from the rulebook:

```go
var fixtureDimensions = []string{"availability", "execution", /* ... */ "tool-description-efficacy"}
```

Before, the `-0` test used a grade request, which never writes baseline counts, so `!bytes.Contains(out, ":-0")` always held. After, it uses an anchor request that carries the entry into the output, spells `-0` in the raw input bytes, and asserts `"k":0` is present.

## Related

- PR #6, which adds the judge and these tests.
