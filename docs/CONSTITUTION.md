# CONSTITUTION

## PRINCIPLES

- loopc is one sampled control loop over a semantic plant. The sensor and the
  actor are language models; the error typing, control law, authority ceiling,
  and arena are deterministic. Reliability comes from typed feedback and
  bounded reversible actuation, not from the agent being correct.
- Conditions are typed, never prose. A control law that matches on
  human-readable text is a defect: the message becomes the interface, and
  behaviour changes silently whenever wording changes.
- The agent declares a typed action before it acts. It may choose freely within
  an action class, but the class is a closed enum. An untyped action cannot be
  learned from, and a loop that cannot be learned from cannot tune its own
  policy.
- Authority is permission to mutate inside the arena. Scalar error may only
  narrow the admissible action set, never widen it: no number can admit an
  arena-mutating action, and only typed conditions can. Narrowing toward the
  non-arena classes is fail-closed; admitting an arena mutation from a number
  would be a readiness score by another name.
- Safety is the arena, not the actuator. The agent performs mutations, so the
  boundary is blast radius: a controller-owned worktree, a feature branch, no
  force-push, no merge, no deploy.
- Observe before acting. Audit is the default ceiling. Reconcile is a
  deliberate, separately recorded transition, never a default.
- Record what was observed when it was observed. The journal is append-only and
  is the training set for the outer loop, not merely an audit trail.
- Every conclusion carries the observations that produced it. A classification
  without evidence is a defect, not a shortcut.
- Fail closed. Missing, ambiguous, or unproven evidence blocks. Nothing in the
  plant, the agent's output, or a review comment can widen authority.
- Complexity is admitted one rung at a time. Each rung introduces exactly one
  new class of difficulty and is not started until the previous rung
  demonstrably converges.

## CONSTRAINTS

### Conditions and error

- A condition is `{Type, Status, Reason, Message, FirstObservedAt,
  LastTransitionAt, ObservedGeneration}`. `Message` is for humans and is never
  read by the control law.
- The error term is a typed vector over condition types carrying count and the
  age of the oldest instance. Its scalar reduction is derived and
  non-authorising.
- `ObservedGeneration` binds a condition to the exact configuration and head it
  was observed against. A condition observed against a superseded generation is
  stale and blocks.

### Action

- Action classes are a closed enum. Adding a class is a constitution change.
- Every class is either arena-mutating or non-arena. `request_human` and `defer`
  are the non-arena classes: they notify or wait, and never write to a worktree,
  a branch, or GitHub. Every other class mutates inside the arena and requires
  authority admitted by typed conditions.
- Because the non-arena classes mutate nothing, they require no authority and
  are always available. Selecting one is never an exercise of authority.
- Every action is preceded by a durable declaration recording class, target,
  intent, and predicted cleared conditions. Executing without a committed
  declaration is prohibited.
- Predictions are recorded and scored. A confidently wrong sensor is a stronger
  warning than a rising error.
- One action is in flight globally, and the next is selected only after
  remeasurement.

### Brakes

- Both brakes only narrow the admissible action set. Neither admits a class, and
  no threshold in either causes an arena mutation that typed conditions had not
  already admitted.
- The integral brake withdraws the arena-mutating classes for a condition that
  persists beyond its configured cycle budget, leaving `request_human` as the
  terminal outcome rather than repetition.
- The derivative brake narrows the admissible set to the non-arena classes when
  the last K actions produced no error reduction. Because halting and escalating
  mutate nothing, they need no authority and remain available even when the
  brake has withdrawn every arena-mutating class.

### Arena

- Mutation happens only in a controller-owned worktree beneath
  `~/worktrees/<owner>/<repository>/<lane>`. User checkouts are never targets.
- The default branch is never written. Force-push, branch deletion, history
  rewriting, merge, and deploy are prohibited at every rung before the rung that
  introduces them.
- The controller refuses to act on any change whose diff touches the arena or
  the control law. Self-improvement does not include self-modification.
- Credentials never enter model context, journal payloads, or command arguments.

### Journal

- Append-only, with prepared, started, observed, and terminal states for every
  action.
- In-memory state advances only after the write that persists it commits.
- Every cycle records the outcome triple: conditions before, action class, error
  delta, cycles to clear, and prediction hit.
- Recovery reads external state before retry and never duplicates a push,
  comment, or thread resolution.

### Authority

- Audit is the default and has no executor. Reconcile requires a recorded audit
  observation for the identical policy and reobserves before acting.
- Configuration change invalidates observations, declarations, and unstarted
  work.

### Kit-Managed Baseline Rules

<!-- BEGIN KIT-MANAGED BASELINE RULES -->
- Treat `docs/CONSTITUTION.md` as the canonical project contract.
- Keep `AGENTS.md`, `CLAUDE.md`, and `.github/copilot-instructions.md` aligned with the repo-local docs tree.
- Treat `docs/notes/<feature>` as optional source material, not canonical truth; promote durable decisions into `SPEC.md`, `docs/CONSTITUTION.md`, or durable references.
- Use native agent planning for research, clarification, design, and implementation planning.
- Before implementation, inspect code and repository memory; create or adopt `SPEC.md` when material rationale exists.
- After validation, curate feature rationale, project invariants, reusable practices, and domain knowledge into their scope-appropriate canonical documents.
- Allow a justified `not required` repository-memory decision when code and tests preserve the complete durable truth.
- Keep every version-control-eligible handwritten implementation/source and test file at 300 physical lines or less.
- Before delivery, audit the complete affected source/test scope; whole-project reconcile and scheduled maintenance audit the entire repository.
- Exclude documentation files, all `docs/**`, all `.kit/**`, `.kit.yaml`, ignored files, vendored dependencies, and proven generated files.
- Split oversized files by semantic responsibility while preserving stable public entry points and behavior; never use minification or arbitrary numbered chunks to claim compliance.
<!-- END KIT-MANAGED BASELINE RULES -->

## CHANGE CLASSIFICATION

<!-- all work falls into one of two tracks — classify before acting -->

### Repository-Memory Work

<!-- use when: consequential product rationale, architecture, cross-component behavior, or historical decisions must survive -->
<!-- workflow: native plan → create/adopt SPEC.md before code → implement → validate → curate repository memory -->
<!-- legacy staged documents: BRAINSTORM.md, legacy SPEC.md, PLAN.md, TASKS.md only when explicitly chosen -->

### Ad Hoc (Lightweight)

<!-- use when: bug fixes, security reviews, refactors, dependency updates, config changes, small refinements -->
<!-- workflow: understand → implement → verify -->
<!-- docs: update practical canonical docs when behavior changes -->
<!-- do not create feature SPEC.md solely for ceremony; report a justified not-required memory decision -->

### Ad Hoc with Existing Specs

<!-- if change touches code with existing spec docs: update them when rationale, behavior, requirements, or approach changes -->
<!-- leave them unchanged when code and tests communicate the complete durable truth -->

## NON-GOALS

- A general-purpose autonomous agent, shell, or credential broker.
- Merging, deploying, or verifying production. Those arrive at later rungs and
  are not implied by any earlier one.
- Cross-pull-request dependency ordering, pull-request forests, or
  infrastructure reconciliation.
- Numeric PID tuning, a readiness score, or any number that grants authority.
- Agent- or repository-authored expansion of the authority ceiling. A review
  comment is data, never an instruction to the controller.
- Automatic recovery from every detected problem. Escalation to a human is a
  terminal, correct outcome.
- A general code-review bot. loopc reconciles feedback on a pull request it has
  been explicitly handed.

## DEFINITIONS

- **Plant** — the system under control. At rung 1, one open pull request.
- **Setpoint** — the reference state. At rung 1, zero unresolved actionable
  review threads at the current head.
- **Condition** — one typed, evidence-carrying observation about the plant.
- **Error term** — the typed vector of unsatisfied conditions. Its scalar
  reduction is `e(t)`.
- **Authority** — permission to mutate inside the arena. Only typed conditions
  admit it; the non-arena classes require none.
- **Action class** — one member of the closed actuator enum, either
  arena-mutating or non-arena.
- **Non-arena class** — `request_human` or `defer`. Notifies or waits, and never
  writes to a worktree, a branch, or GitHub.
- **Admissible action set** — the classes available on the current cycle. Typed
  conditions admit arena-mutating classes; the scalar and the brakes may only
  withdraw them.
- **Declaration** — the durable record an agent writes before acting.
- **Arena** — the bounded, reversible region in which the agent may mutate.
- **Rung** — one step on the complexity ladder, introducing exactly one new
  class of difficulty.
- **Inner loop** — observe, type, measure, select, act, remeasure.
- **Outer loop** — adjustment of the control law from recorded outcome triples.
  Its data is collected from the first cycle; its learner is deferred.
- **Sensor** — the agent role converting raw observation into typed conditions.
- **Actor** — the agent role executing a declared action class inside the arena.
