# Changelog

All notable changes will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project intends to use [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once releases begin.

## [Unreleased]

### Added

- Initial modular-monolith, Angular SPA, PostgreSQL schema, OpenAPI contract, deterministic seed, localization, Docker, benchmark, and documentation work in progress.
- Bilingual portfolio README, architecture and case study; threat model, benchmark and competitor protocols, dependency inventory, demo/release guidance, and evidence-only screenshot workflow.
- First-class English/Russian catalogs, locale-preference hierarchy, translator checks, and a tenant-owned content translation workflow with coverage and draft/published states.
- GitHub Actions definitions for lint/typecheck, backend/frontend/integration/E2E tests, production builds, dependency auditing, CodeQL, container scanning/SBOM, PR benchmark smoke, and manually dispatched full benchmarks.
- Contribution, conduct, security, issue, pull-request, and dependency-update policies.

### Security

- CI policy definitions include forced generated-state checks, forbidden dependency checks, `govulncheck`, production `pnpm audit`, license review, CodeQL, Trivy, and an SPDX SBOM.

> Workflow definitions have been added, but their successful execution on GitHub-hosted runners is not claimed here. Consult workflow runs and `docs/STATE.md` for verified status.
