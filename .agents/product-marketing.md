# Product Marketing Context

**Document version:** v1
**Last updated:** 2026-07-21

## Product Overview

**One-liner:** An open-source, resource-efficient Sales CRM for small and medium teams that want strong tenant isolation and reproducible performance evidence without a large infrastructure footprint.
**What it does:** Manages customer relationships, sales pipelines, activities, collaboration, reports, and practical automations in a multilingual SPA. Its base deployment is one application container plus PostgreSQL.
**Product category:** Sales CRM / self-hosted CRM.
**Product type:** Open-source web application and self-hosted SaaS foundation.
**Business model:** Open source under MIT; hosted or commercial plans are not defined.

## Target Audience

**Target companies:** Small and medium organizations and technical teams that value self-hosting, predictable resource use, and auditable security boundaries.
**Decision-makers:** Founders, heads of sales, RevOps/Sales Ops leaders, and technical owners.
**Primary use case:** Run a complete sales workflow with contacts, companies, deals, activities, automation, search, and reporting on modest infrastructure.
**Jobs to be done:**
- Keep sales data and follow-ups consistent across a team.
- Move opportunities through a measurable pipeline without maintaining many infrastructure services.
- Verify tenant isolation, performance, and operational claims through reproducible tests.
**Use cases:** Self-hosted sales operations, multi-workspace CRM, portfolio-quality reference implementation, and low-resource deployments.

## Personas

| Persona | Cares about | Challenge | Value we promise |
| --- | --- | --- | --- |
| Sales representative | Fast daily workflow | Slow navigation and fragmented follow-ups | Responsive lists, pipeline, timeline, tasks, and search |
| Sales manager | Forecast and consistency | Unclear pipeline health | Focused dashboards, reports, audit history, and automation |
| Technical owner | Security and operability | Infrastructure sprawl and unverifiable claims | PostgreSQL-centered design, RLS, documented tests and benchmarks |
| Workspace owner | Control | Cross-tenant risk and weak permissions | RBAC, scoped access, sessions, audit, and defense-in-depth RLS |

## Problems & Pain Points

**Core problem:** Teams need practical CRM depth without high operational complexity or a sluggish interface.
**Why alternatives fall short:** Some options require multiple infrastructure services, hide performance methodology, or are too broad for focused sales work. These are design hypotheses, not measured competitor findings.
**What it costs them:** Administrative time, missed follow-ups, unreliable reporting, and infrastructure overhead.
**Emotional tension:** Fear of losing customer context, missing revenue, leaking tenant data, or adopting a system that becomes expensive to operate.

## Competitive Landscape

**Direct:** Other open-source Sales CRMs — no comparative measurement has been performed.
**Secondary:** Hosted commercial CRMs — no comparative measurement has been performed.
**Indirect:** Spreadsheets and ad-hoc databases — flexible initially but difficult to govern, audit, and automate consistently.

## Differentiation

**Key differentiators:**
- Base runtime of one Go application plus PostgreSQL.
- Application guards plus PostgreSQL RLS for tenant isolation.
- Reproducible dataset, browser, load, bundle, and resource measurement protocols.
- RU/EN from the first release with extensible translation catalogs.
**How we do it differently:** PostgreSQL also supplies search, outbox, and the job queue; Angular is zoneless and feature-lazy; the production Go binary embeds the SPA.
**Why that's better:** Fewer moving pieces, smaller operational surface, and claims that can be independently checked.
**Why customers choose us:** Not established; customer research has not been performed.

## Objections

| Objection | Response |
| --- | --- |
| Can a two-container profile support real CRM workflows? | The repository includes repeatable load and resource tests; only measured results will be claimed. |
| Is multi-tenancy safe? | Every object operation has application authorization and tenant-scoped PostgreSQL RLS, with negative integration tests. |
| Is localization bolted on? | Locale, message keys, translation completeness, and user/workspace preferences are core contracts. |

**Anti-persona:** Enterprises needing a full ERP, accounting suite, call-center platform, or undisclosed proprietary integrations out of the box.

## Switching Dynamics

**Push:** Slow workflows, infrastructure sprawl, weak auditability, and lost sales context.
**Pull:** Focused CRM breadth, self-hosting, transparent architecture, and reproducible evidence.
**Habit:** Existing CRM data, integrations, trained workflows, and spreadsheet familiarity.
**Anxiety:** Migration effort, feature coverage, security, and whether resource goals hold under real data.

## Customer Language

**How they describe the problem:** No verified verbatim customer research yet.
**How they describe us:** No verified verbatim customer research yet.
**Words to use:** designed for low-resource deployments; measured under the documented profile; reproducible; tenant-isolated; focused Sales CRM.
**Words to avoid:** fastest; zero-latency; enterprise-ready; infinitely scalable; beats named competitors.
**Glossary:**

| Term | Meaning |
| --- | --- |
| Workspace | Tenant boundary containing CRM data, members, settings, and locale defaults |
| Vertical slice | Fully working user flow across database, API, UI, tests, and deployment |

## Brand Voice

**Tone:** Professional, calm, evidence-led.
**Style:** Direct, concise, technically transparent, localized rather than mechanically translated.
**Personality:** Fast, pragmatic, trustworthy, focused, open.

## Proof Points

**Metrics:** Not measured.
**Customers:** None claimed.
**Testimonials:** None claimed.
**Value themes:**

| Theme | Proof |
| --- | --- |
| Low operational complexity | Architecture and Compose profile, once implemented |
| Tenant isolation | RLS/application negative tests, once passing |
| Performance | Published raw benchmark artifacts, once measured |

## Goals

**Business goal:** Deliver a credible open-source CRM and engineering case study.
**Conversion action:** Clone the repository, run the documented demo, and evaluate it with the included protocols.
**Current metrics:** Not measured.

## Changelog

- v1 (2026-07-21) — Initial context auto-drafted from the master requirements; unverified market and customer assumptions are explicitly marked.

