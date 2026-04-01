## What Changed

<!-- Describe the changes in this PR. What problem does it solve? -->

## Why

<!-- Explain the motivation behind this change. Link to relevant issues. -->

Closes #<!-- issue number -->

## How to Test

<!-- Describe how reviewers can verify the changes. -->

1. <!-- Step 1 -->
2. <!-- Step 2 -->
3. <!-- Step 3 -->

## Screenshots (if applicable)

<!-- Add screenshots for UI changes. -->

## Checklist

### General
- [ ] I have read the [contributing guidelines](../CONTRIBUTING.md)
- [ ] My changes follow the project's coding conventions
- [ ] I have added tests that prove my fix/feature works
- [ ] All new and existing tests pass locally
- [ ] I have updated documentation as needed

### Rust (if applicable)
- [ ] No `unwrap()` in production code; `Result` types used with proper error handling
- [ ] All `unsafe` blocks have justification comments
- [ ] `cargo clippy` passes with no warnings
- [ ] `cargo fmt` has been applied

### Go (if applicable)
- [ ] `context.Context` is propagated for cancellation and tenant isolation
- [ ] Structured logging is used (`slog` or equivalent)
- [ ] `go vet` passes with no issues
- [ ] Table-driven tests are used where appropriate

### TypeScript (if applicable)
- [ ] No `any` types in production code
- [ ] Strict TypeScript mode compliance
- [ ] Design tokens from `packages/design-tokens` are used for styling
- [ ] Components follow progressive disclosure pattern

### Security
- [ ] No secrets, credentials, or API keys are committed
- [ ] Changes maintain five-layer security model
- [ ] Event sourcing audit trail is preserved for all autonomous actions
