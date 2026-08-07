# PROJECT PROGRESS SUMMARY

## FEATURE PROGRESS TABLE

| ID | FEATURE | PATH | PHASE | PAUSED | CREATED | SUMMARY |
| -- | ------- | ---- | ----- | ------ | ------- | ------- |
| 0001 | pr-feedback-reconciliation | `docs/specs/0001-pr-feedback-reconciliation` | ready | no | 2026-08-07 | Rung 1 control loop driving a pull request's actionable review threads to zero |

## PROJECT INTENT

- `loopc` is a control loop for goals that do not reduce to numbers. The sensor
  and the actor are language models; the error typing, control law, authority
  ceiling, and arena are deterministic.
- The project exists to test one falsifiable claim: that reliability in an
  agentic loop comes from typed feedback and bounded reversible actuation rather
  than from the agent being correct.
- Complexity is admitted one rung at a time along a ladder whose terminus is a
  pull request accepted in production. Rung 1 regulates a single pull request's
  review feedback and has no irreversible actuator.
- No product behavior is implemented yet. The repository holds repository-memory
  scaffolding, agent instruction entrypoints, the canonical `Makefile` command
  interface, and the feature contract.

## GLOBAL CONSTRAINTS

- `docs/CONSTITUTION.md` is the canonical project contract; read it before
  reconciling feature docs and do not duplicate its rules here.
- `make help` is the canonical entrypoint for the project command interface.

## FEATURE SUMMARIES

### pr-feedback-reconciliation

- **STATUS**: ready
- **PAUSED**: no
- **INTENT**: Prove that a control loop can reconcile a semantic goal when the
  sensor and actor are language models and the error typing, control law,
  authority ceiling, and arena stay deterministic.
- **APPROACH**: Six pull requests — repository memory, the rung 0 runtime, a
  stability harness against a deterministic fake plant, the rung 1 CodeRabbit
  feedback controller, the CLI and `e(t)` observability, and dogfood evidence on
  loopc's own pull requests.
- **OPEN ITEMS**: Implementation has not begun. Acceptance is AC-001 through
  AC-006 in `SPEC.md`; the principal known risk is an agent/reviewer limit cycle.
- **POINTERS**: `docs/specs/0001-pr-feedback-reconciliation/SPEC.md`

## LAST UPDATED

- 2026-08-07 — established the project contract: constitution principles and
  constraints, the rung 1 living specification, and the README product model.
