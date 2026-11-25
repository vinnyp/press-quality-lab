# Mutation checks

The builder authored the implementation and its tests. Each mutation below was applied separately to `jcs.go`; the named test failed at runtime, and the original source was restored.

| Mutation | Failing test |
|---|---|
| UTF-8 key order | `TestVectors/utf16` |
| Lower decimal threshold | `TestVectors/number-1e-6` |
| Upper decimal threshold | `TestVectors/number-1e20` |
| Negative zero | `TestVectors/number--0` |
| Exponent plus sign | `TestVectors/number-1E\+30` |
| Shortest digits | `TestVectors/number-0.10` |
| Decimal round-trip rejection | `TestErrors/integer-loss` |
| Overflow rejection | `TestErrors/overflow` |
| Unsupported value rejection | `TestErrors/unsupported` |
| Invalid string rejection | `TestErrors/invalid-string` |
| Invalid key rejection | `TestErrors/invalid-key` |
| Cycle rejection | `TestErrors/.*-cycle` |
| Number grammar rejection | `TestErrors/leading-zero` |
| Control escape | `TestVectors/escapes` |
| Escape \b | `TestVectors/escapes` |
| Escape \f | `TestVectors/escapes` |
| Escape \n | `TestVectors/escapes` |
| Escape \r | `TestVectors/escapes` |
| Escape \t | `TestVectors/escapes` |
| Quote escape | `TestVectors/escapes` |
| Backslash escape | `TestVectors/escapes` |
