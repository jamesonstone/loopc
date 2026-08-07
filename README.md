# loopc
🔄 PID-driven Process Control.

<!-- BEGIN KIT-MANAGED README BADGES -->
[![Last commit](https://img.shields.io/github/last-commit/jamesonstone/loopc)](https://github.com/jamesonstone/loopc/commits) [![Open issues](https://img.shields.io/github/issues/jamesonstone/loopc)](https://github.com/jamesonstone/loopc/issues) [![Pull requests](https://img.shields.io/github/issues-pr/jamesonstone/loopc)](https://github.com/jamesonstone/loopc/pulls) [![Release](https://img.shields.io/github/v/release/jamesonstone/loopc)](https://github.com/jamesonstone/loopc/releases)
<!-- END KIT-MANAGED README BADGES -->

loopc is a control loop for goals that do not reduce to numbers. The sensor and
the actor are language models. The error typing, control law, authority ceiling,
and arena are deterministic. Reliability comes from typed feedback and bounded
reversible actuation, not from the agent being correct.

## Product model

```text
observe -> type conditions -> compute e(t) -> select action class -> act in arena -> remeasure
```

Conditions are typed records, never prose, and the control law reads only their
types. The scalar error `e(t)` drives trend, plots, and brakes; it may remove
authority but never grant it. Before acting, the agent commits a declaration
naming a closed-enum action class, its target, its intent, and the conditions it
expects to clear — so that every cycle yields a learnable outcome.

Two brakes bound the loop. The integral brake escalates a condition that
persists past its cycle budget, terminating in `request_human` rather than
repetition. The derivative brake halts when recent actions have stopped reducing
error.

## Rung 1

Complexity is admitted one rung at a time, on a ladder whose terminus is a pull
request accepted in production. The first rung regulates one open pull request,
with zero unresolved actionable review threads at the current head as its
setpoint. It has no irreversible actuator: the loop drives feedback to zero and
stops, never merging.

Safety is blast radius rather than a deterministic actuator. Every mutation
happens in a controller-owned worktree on a feature branch — never the default
branch, never a force-push, never a merge or deploy. The controller refuses to
act on a change that touches its own arena or control law.

Audit is the default ceiling and has no executor. Reconcile is a deliberate
transition that requires a recorded audit observation for the identical policy.

## Documentation

- [Constitution](docs/CONSTITUTION.md) — project invariants
- [Living specification](docs/specs/0001-pr-feedback-reconciliation/SPEC.md) —
  the ladder, the falsifiable claim, and material decisions

## Maintainers

Maintained with 🪖 and ❤️ by [Jameson](https://github.com/jamesonstone) (`jamesonstone`).
