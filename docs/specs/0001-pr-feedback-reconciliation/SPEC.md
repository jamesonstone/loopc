---
kit_metadata_version: 1
artifact: "spec"
workflow_version: 3
phase: "ready"
feature:
  id: "0001"
  slug: "pr-feedback-reconciliation"
  dir: "0001-pr-feedback-reconciliation"
clarification:
  status: "resolved"
  confidence: 95
  unresolved_questions: 0
references:
  - id: "feature-notes"
    name: "Feature notes"
    type: "notes"
    target: "docs/notes/0001-pr-feedback-reconciliation"
    relation: "informs"
    read_policy: "conditional"
    used_for: "optional pre-brainstorm research input"
    status: "optional"
  - id: "issue-3"
    name: "loopc control-loop contract and rung 1 scope"
    type: "issue"
    target: "https://github.com/jamesonstone/loopc/issues/3"
    relation: "guides"
    read_policy: "must"
    used_for: "scope and acceptance criteria"
    status: "active"
  - id: "constitution"
    name: "loopc constitution"
    type: "reference"
    target: "docs/CONSTITUTION.md"
    relation: "constrains"
    read_policy: "must"
    used_for: "control-loop invariants, arena boundary, authority ceiling"
    status: "active"
delivery_intent: issue_branch_pr_ready
---
# SPEC

## PURPOSE

Prove one falsifiable claim on the smallest plant that still lies on the path to
"pull request merged to production":

> A control loop can reliably reconcile a semantic goal when the **sensor** and
> **actor** are language models while the **error typing, control law, authority
> ceiling, and arena** stay deterministic. Reliability comes from typed feedback
> and bounded reversible actuation, not from the agent being correct.

The claim is refuted if the loop limit-cycles and cannot be damped, if `e(t)`
does not converge, or if the semantic sensor's classifications are not stable
enough across cycles to control on.

## CONTEXT

`ghostgc` and `merge-controller` independently converged on the same program: a
local Go daemon, an audit/shadow ceiling promoted to reconcile, evidence-bound
fail-closed gates, an append-only ledger, one action per cycle, a two-sample
deadband, exact-identity binding, and a `status`/`logs`/`doctor`/`service`
surface. They also duplicate a domain outright — roughly 4,000 lines of git
worktree lifecycle in `ghostgc/internal/worktree` plus `daemon/worktree_*.go`
against `merge-controller/internal/localmaintenance` and
`internal/treeworkspace` — with divergent reversibility semantics: `ghostgc`
retires, waits out a grace period, then finalises in a separate foreground
approval, while `merge-controller` performs a one-phase removal once GitHub
proves the pull request merged.

Both take too large a bite, but along different axes.

`merge-controller` regulates the wrong plant for a first controller. Its system
has dead time measured in minutes (CI), a noisy sensor (flaky checks and an
eventually-consistent GitHub API), and an irreversible actuator (merge, deploy).
`internal/productioncontrol` is its largest package at 3,654 lines and covers
ECS task sets, image digests, CloudFront invalidations, S3 object inventories,
and rollback leases.

`ghostgc` has near-ideal plant physics — local sub-second measurement, low
sensor noise, and actuation that is reversible by construction (quarantine is a
rename, retirement is a move, finalisation is a separate later approval) — but
runs four plants in one daemon: processes, sessions, cache artifacts, and
worktrees.

Critically, neither emits an error signal.
`merge-controller/internal/prforest/equilibrium.go` reduces the entire plant to
`Reached bool` plus `Blockers []string`, and `internal/prforest/selector.go`
implements the control law as `strings.Contains` over a joined blob of English
blocker sentences. `internal/control/engine.go` carries `ActiveTrees` and
`BlockedTrees` in its `Measurement` but uses only `ActiveTrees > 0` to choose a
poll interval, so no time series is retained.

Both projects reject a "numeric readiness score" in their constitutions, and
they are right to. The resolution adopted here is that the control law reads
only the typed condition vector, while the scalar `e(t)` exists for trend,
plots, and brakes — and may only ever *remove* authority.

Neither codebase is a dependency. Both are references whose discipline is ported
deliberately and whose limitations are recorded as lessons.

## REQUIREMENTS

- One sampled inner loop: observe, type conditions, compute `e(t)`, select an
  action class, act inside the arena, remeasure.
- Conditions are typed records carrying `Type`, `Status`, `Reason`, `Message`,
  `FirstObservedAt`, `LastTransitionAt`, and `ObservedGeneration`. The control
  law never reads `Message`.
- The error term is a typed vector over condition types with count and age of
  the oldest instance. Its scalar reduction never authorises an action.
- The agent commits a typed declaration — class, target, intent, predicted
  cleared conditions — before executing anything.
- Action classes are the closed enum `patch_code`, `patch_test`, `patch_docs`,
  `reply_no_change`, `request_human`, `defer`.
- The journal is append-only and records the outcome triple for every cycle:
  conditions before, action class, error delta, cycles to clear, prediction hit.
- The integral brake escalates a condition that persists past its cycle budget;
  the terminal escalation is `request_human`.
- The derivative brake halts the loop when the last K actions produced no error
  reduction.
- The arena confines every mutation to a controller-owned worktree on a feature
  branch, with no force-push, no merge, and no deploy.
- The controller refuses to act on a pull request whose diff touches the arena
  or the control law.
- Audit is the default ceiling and has no executor. Reconcile requires a
  recorded audit observation for the identical policy.
- The agent runtime sits behind an adapter interface so the model is
  substitutable.

### Non-goals

- Rungs 2 through 6: CI as a second condition type, ambiguous human feedback,
  merge, cross-pull-request dependency ordering, and deployment verification.
- The outer loop's learner. Its data is recorded from the first cycle; nothing
  learns from it yet.
- Any numeric tuning, readiness score, or scalar that grants authority.
- Reuse of `merge-controller/internal/productioncontrol`, which returns at rung
  6 or not at all.

### Observable acceptance

- **AC-001** — The stability suite passes against a deterministic fake plant,
  covering settling, oscillation, steady-state escalation, disturbance
  rejection, windup, and the derivative stop.
- **AC-002** — `loopc run --once` in audit mode emits typed conditions and a
  computed `e(t)` and takes no action.
- **AC-003** — Reconcile mode is refused without a recorded audit observation
  for the identical policy.
- **AC-004** — A cycle that acts writes a declaration before the action and an
  outcome triple after it, both durable and append-only.
- **AC-005** — The controller refuses a pull request whose diff touches
  `internal/arena` or `internal/policy`.
- **AC-006** — On loopc's own pull requests, unresolved actionable CodeRabbit
  threads converge toward zero, and the `e(t)` series is exportable.

## ACCEPTED PLAN

The plant is chosen by physics, not ambition: fast local measurement, low sensor
noise, reversible actuation, a naturally ordered error term, irreducible
semantic content, and frequent natural disturbances.

**Rung 1** — one open pull request in loopc's own repository. Setpoint is zero
unresolved actionable CodeRabbit threads at the current head. The sensor
classifies each thread `actionable`, `obsolete`, `informational`, or
`environmental`. The actor is Codex behind an adapter. There is no irreversible
actuator: the loop drives to "feedback addressed" and stops, never merging.
CodeRabbit re-reviews on every push, supplying the disturbance for free.

The expected first failure is a limit cycle — the agent fixes A, breaks B, and
CodeRabbit flags B. Damping it is the result worth having.

Delivery proceeds in six pull requests:

1. Repository memory: constitution, this spec, README product model.
2. Rung 0 runtime: `condition`, `action`, `journal`, `trend`, `policy`, `arena`,
   the agent interface, and `runtime`. No domain, no network.
3. Stability harness: deterministic fake agent and fake plant plus the six
   stability tests. Deliberately before the real agent, so the first evidence
   costs nothing and instability stays attributable to the loop rather than the
   model.
4. Rung 1 controller: bounded `gh` wrapper, the `prfeedback` controller, and the
   Codex adapter.
5. CLI and observability, including `e(t)` export.
6. Dogfood evidence against loopc's own pull requests.

The ladder beyond rung 1, each rung adding exactly one class of difficulty:
rung 2 adds CI (slow, noisy sensor); rung 3 adds human feedback (ambiguity and
non-code action classes); rung 4 adds merge (irreversibility, and the return of
a deterministic executor for that one action); rung 5 adds a second pull request
(coupling and ordering); rung 6 adds deployment and production verification (an
external plant).

## DECISIONS

### The plant is chosen for its physics, not its ambition

"Pull request merged to production" remains the terminus, but it is the worst
possible first plant: long dead time, noisy sensors, irreversible actuation.
Rung 1 keeps the trajectory while removing every one of those properties.
Rejected: starting from `merge-controller`'s existing scope and shrinking it,
which preserves the bad physics.

### Typed conditions replace blocker prose

`merge-controller` proved that an unbounded plant collapses its error term into
English. Kubernetes-style conditions are adopted instead, giving persistence and
transition times for free. Rejected: keeping `[]string` blockers with a stricter
matcher, which leaves the message as the interface.

### The scalar may remove authority but never grant it

This reconciles emitting `e(t)` with both predecessor constitutions' rejection
of a readiness score. Trend, plots, and the derivative brake read the scalar;
only typed conditions authorise. Halting on a number is fail-closed.

### The agent acts, so safety moves to the arena

`merge-controller` guaranteed that deterministic code owned all mutation. Rung 1
gives that up so the agent can explore feedback and responses at small scale.
The replacement guarantee is blast radius: controller-owned worktree, feature
branch, no force-push, no merge, no deploy. The deterministic executor returns
at rung 4, when the first irreversible actuator appears.

### Actions are typed even though the agent takes them

Recursive self-improvement here means the controller tuning its own policy from
outcome data. A policy cannot be learned over an untyped action space, so the
agent declares a closed-enum class before acting and remains free within it.
This single contract is what keeps the outer loop reachable.

### Predictions are recorded and scored

Declaring which conditions an action is expected to clear costs almost nothing
and yields a calibration signal. A confidently wrong sensor is an earlier and
sharper warning than a rising error.

### The outer loop's schema ships before its learner

The journal schema cannot be retrofitted onto cycles already run; the learner
can be added at any time. Outcome triples are therefore recorded from the first
cycle while the shipped policy stays fixed and hand-written.

## DISCOVERIES

None yet. This pull request delivers repository memory only and ran no
implementation. Consequential discoveries are recorded here as each subsequent
pull request lands.

## VALIDATION

Repository-memory delivery for this pull request:

- `make check` — Kit project document and instruction contract.
- `docs/CONSTITUTION.md` contains no `TODO` markers.
- The complete branch diff is documentation-only and qualifies for the Kit
  `[skip ci]` directive; the repository has no branch protection, its single
  ruleset is disabled, and `auto-assign.yml` is not a required check.

Implementation validation is defined by AC-001 through AC-006 and is recorded as
each subsequent pull request lands.

## OUTCOME

This pull request establishes the contract only: constitution invariants, this
spec, and the README product model. No runtime exists yet.

The rung 0 runtime, the stability harness, the rung 1 controller, the CLI, and
dogfood evidence follow in sequence, and this section records what was actually
built, remaining risk, and any divergence from the accepted plan as they land.

The principal known risk is the limit cycle named in the accepted plan: the
agent fixing one thread while creating the condition for another. The stability
harness exists specifically to detect it against a fake plant before the real
agent is wired in.

## REPOSITORY MEMORY

- Created `docs/CONSTITUTION.md` principles, constraints, non-goals, and
  definitions. These are project invariants rather than feature rationale: they
  bind every rung, not just rung 1.
- Created this spec as the living record of the feature's rationale, the
  complexity ladder, and the decisions that code and tests cannot preserve.
- Updated `README.md` with a product model beneath the existing tagline.
- `docs/notes/0001-pr-feedback-reconciliation/` exists as optional source
  material and is not canonical.
