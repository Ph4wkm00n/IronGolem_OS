# Rust Runtime Domain

The Rust runtime is the **trusted execution core** of IronGolem OS. All plan
execution, policy enforcement, and state management flows through Rust.

## Purpose

- Trusted execution of plans and workflows
- Policy enforcement adapters
- Secure tool orchestration within sandboxes
- Checkpointing and rollback management
- Memory graph write-path
- Verifier and evaluator execution
- WASM plugin hosting

## Key Submodules

### Plan Engine (`runtime/core/`)
Executes plan graphs - the directed acyclic graphs that represent agent
workflows. Each node in a plan graph is a step (tool call, LLM call, approval
gate, delegation).

### Execution State Machine (`runtime/workflow/`)
Manages the lifecycle of plan execution: pending, running, paused, completed,
failed, rolled-back. Ensures deterministic state transitions.

### Checkpoint Manager (`runtime/checkpoints/`)
Snapshots execution state at configurable intervals. Enables:
- Resumption after crashes
- Rollback to known-good states
- Replay for debugging and auditing

### Rollback Manager (`runtime/checkpoints/`)
Restores system to a previous checkpoint. Used by the self-healing loop when
automated recovery strategies fail.

### Verifier Runtime (`runtime/verifier/`)
Executes quality-gate checks on agent outputs before they proceed. Catches
hallucinations, policy violations, and format errors.

### Sandbox Host (`runtime/sandbox/`)
Isolates tool execution. Each tool call runs in a constrained environment with
resource limits and capability restrictions.

### Risk Primitives
Metadata types for risk scoring, propagated through every action. Used by the
policy engine and defense module to make enforcement decisions.

### WASM Plugin Host
Stub for future extensibility. Third-party tools and custom logic can be
compiled to WASM and run inside the sandbox.

## Design Principles

- No `unwrap()` in production - all errors handled explicitly
- All `unsafe` blocks require justification comments
- Checkpoint-first: state must be recoverable after any failure
- Risk metadata propagates through every plan step
- Memory graph writes are append-only with event sourcing

## Canonical Reference

See the Rust runtime domain section in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
