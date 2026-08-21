# Dynamic Guardrail Specification

## Principle

Agent behavior, not repeated count, determines enforcement level. One fake claim
is enough to trigger critical enforcement.

## Behavior table

| Behavior | Required evidence | Violation | Enforcement |
|---|---|---|---|
| Claims tests passed | Fresh `test` or `smoke` command success in same epoch | `fake_test_claim` | Smoke takeover + block writes |
| Claims build passed | Fresh `check`, `build`, or `smoke` success in same epoch | `fake_build_claim` | Smoke takeover + block writes |
| Claims no errors | Latest diagnostics must have zero blocking errors, or fresh successful check/build evidence | `fake_no_errors_claim` | Smoke takeover + block writes |
| Claims work done | Current epoch must contain file-change, command, or diagnostic evidence | `unsupported_done_claim` | Smoke takeover + block writes |
| Diagnostics regress | Blocking diagnostic count increases | `diagnostics_regression` | Evidence required; can be promoted by policy |

## Epoch

An epoch is the ID of the latest workspace snapshot. Evidence is only accepted
when it belongs to the same epoch and no file change happened after the evidence.

## Fail-closed rule

If a claim or diagnostic payload cannot be decoded, YKC treats the event stream
as unsafe and enters smoke takeover mode.
