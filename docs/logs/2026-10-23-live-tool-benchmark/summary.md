# Real-model read/tool benchmark

On 2026-10-23 (Asia/Shanghai), the running minimal-chat candidate passed all
twelve task samples: six cases repeated twice, through the configured
`Gemini 3.8 local gateway` / `gemini-3.8-flash-tiered` route. Every sample
has a correct synthetic answer, the required successful file reads, and
exactly one completed terminal event. This is a new reproducible baseline;
the original benchmark script/results were not found, so no improvement or
regression relative to that earlier run is claimed.

| Metric | Observed value |
| --- | ---: |
| Passed task samples | 12 / 12 |
| End-to-end median | 10.957 s |
| End-to-end mean | 13.900 s |
| End-to-end range | 6.540–29.782 s |
| Sum of task elapsed times | 166.798 s |
| Mean time to first answer-text delta | 13.479 s |
| Model requests / closures | 39 / 39 |
| Requested / finished tools | 29 / 29 |
| Provider-reported input tokens | 103,309 |
| Provider-reported output tokens | 861 |
| Provider-reported total tokens | 104,170 |
| Provider retries / timed-out runs | 0 / 0 |

The two tool errors are intentional missing-file reads, one per recovery
sample. Both runs continued to read the fallback file and completed correctly.
All 39 model calls closed as completed and reported usage; no missing usage
was synthesized. All twelve runs have one terminal event.

| Task | Passed | Round 1 / 2 elapsed | Round 1 / 2 model calls | Round 1 / 2 tools |
| --- | ---: | ---: | ---: | ---: |
| Read unknown nonce | 2 / 2 | 7.819 / 6.540 s | 2 / 2 | 1 / 1 |
| Follow three-file manifest chain | 2 / 2 | 17.337 / 17.151 s | 4 / 4 | 3 / 3 |
| Aggregate paid CSV orders | 2 / 2 | 10.418 / 10.070 s | 2 / 2 | 1 / 1 |
| Diagnose off-by-one code | 2 / 2 | 27.889 / 29.782 s | 6 / 7 | 7 / 6 |
| Recover from missing input | 2 / 2 | 11.496 / 11.773 s | 3 / 3 | 2 / 2 |
| Follow up with prior context and new file | 2 / 2 | 8.324 / 8.199 s | 2 / 2 | 1 / 1 |

The code-inspection case accounts for 40,631 total tokens and both slowest
samples. It made directory-list calls in addition to reading the two supplied
paths. This is observed extra exploration on these samples, not an established
runtime regression or a new optimization claim.

The executable source is `397b6f17`, the Recipe is minimal-chat, and the
Generation is `1c6fd4a5bf0a9ebfb3f4a0be06465186` (eleven Modules). Only
`list_dir` and `read_file` are active. The benchmark reuses the existing
disposable acceptance backend, creates independent synthetic workspaces and
titled sessions, and records metrics through public RPC. It does not change
provider settings, policy, permissions, the running package, or product code.

`benchmark.mjs` is the rerunnable external client; `results.json` contains
safe per-run projections and synthetic answer checks. See `verification.md`
for measurement definitions and `acceptance.md` for repetition instructions.
The temporary input directories were removed after projection verification.
The runtime and its test-session records remain available for manual use.

Write/patch/shell tasks, approval UI, concurrency, cancellation, restart
recovery, full-product parity and D14 closure are not certified here. No
release notes are included because no runtime artifact is released. There is
no stable-root integration, push or publication.
