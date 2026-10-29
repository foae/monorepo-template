# Go guidance sources

Read this when maintaining the guidance, not as a prerequisite for Go work.

The [task index](conventions.md#read-by-task), [lifecycle rules](patterns.md#context-and-concurrency-ownership) and [change workflows](go-workflows.md) are maintained in this repository so they travel with forks. No upstream skill installation, global agent setup or harness-specific command is required. Add a thin `.agents/skills/` adapter only if it provides a demonstrated discovery or invocation benefit; it must link to these docs rather than duplicate their rules.

## Selection policy

Adopt a procedure only when it fills a local gap, is technically sound, respects the template's architecture and can be loaded for a specific task. Existing conventions and implementation contracts remain authoritative. This selection is based on content review, not a measured improvement in agent performance; upstream benchmark scores do not establish effectiveness here.

Relevant entrypoints and selected references were reviewed, not every reference or every library-specific skill. Selected material comes from [samber/cc-skills-golang at revision 246b6c4c1ab3077dee114beb9ade3bb0527dfebc](https://github.com/samber/cc-skills-golang/tree/246b6c4c1ab3077dee114beb9ade3bb0527dfebc):

| Upstream skills (`golang-` prefix) | Local use |
| --- | --- |
| `context`, `concurrency` | Cancellation, completion and channel ownership |
| `troubleshooting`, `testing` | Focused diagnosis and test-order investigation |
| `refactoring`, `gopls` | Semantic navigation and caller migration |
| `safety`, `structs-interfaces` | Conditional ownership, nil and copy-safety checks |
| `security` | Evidence-based trust-boundary review |

## Deliberate exclusions

- **No general skill router or cross-skill loading chain.** The upstream `golang-how-to` router loads primary and secondary skills together; the local index selects a section instead. Personas, orchestration instructions and harness-specific tool setup do not belong in the shared technical guidance.
- **No alternative architecture.** Layout, design-pattern and dependency-injection skills must not replace the single module, existing layers or manual wiring. Database advice must not bypass sqlc or prohibit the repository's reviewed migration workflow.
- **No duplicated rulebook.** Style, documentation and error-handling advice largely overlaps existing conventions. No new logging dependency or logging-policy change is needed.
- **No blanket safety prescriptions.** Sending pointers can be safe with explicit ownership; concurrent map reads are safe without writes; nil receivers can be intentional; copies are not always necessary. Preserve these distinctions rather than copying upstream absolutes.
- **No testing machinery by default.** Do not introduce mandatory integration tags, coverage thresholds, mock frameworks, leak-check dependencies or per-edit test loops. Keep the repository's deterministic, behavior-focused testing contract and verification gate.
- **No automatic modernization, dependency upgrades or performance tuning.** Those require their own task and evidence. Benchmark methodology may be useful for a performance investigation but is not part of this compact reference.

When updating, review the selected source and relevant references against current Go documentation and local contracts. Record the new source revision only for material actually reviewed. Preserve attribution for substantially adapted material; do not make upstream links a runtime dependency for following the local procedures.

## Upstream license

Adapted material from samber/cc-skills-golang is provided under the following license. The repository's own [license](../LICENSE) remains unchanged.

MIT License

Copyright (c) 2026 Samuel Berthe

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
