# 000 — Repo conventions: Conventional Commits + SemVer

**Status:** Accepted
**Date:** 2026-08-23

## Context

Public from day 1; conventions are easiest to enforce
from the first commit, so they start now.

## Decision

- Every commit message follows Conventional Commits 1.0.0.
- Allowed types: `feat`, `fix`, `docs`, `chore`, `refactor`,
  `test`, `ci`. New types require amending this ADR.
- Every release tag follows SemVer 1.0.0 (`vMAJOR.MINOR.PATCH`).
- Commit types map to version bumps: `fix:` → PATCH,
  `feat:` → MINOR, `BREAKING CHANGE` → MAJOR.
- Until `v1.0.0`, 0.x rules apply: anything may change between
  minor versions. Phase ends cut `v0.<phase>.0` tags.
- CI will eventually enforce the commit format (Phase 1, P1.03
  or later).

## Consequences

- History is machine-readable; changelogs can be generated
  from commits.
- Writing a commit forces me to classify the change first.
- Version tags tell the project's story at a glance.

* Slight friction on every commit — acceptable cost.
