# VeltrixCRM roadmap

**Status: active development.** This roadmap describes intended outcomes, not delivery dates or completed work. The verified implementation state is maintained in [docs/STATE.md](docs/STATE.md); measured results belong in [docs/PERFORMANCE.md](docs/PERFORMANCE.md).

## Current focus — pre-1.0 stabilization

- Resolve critical and high findings from hosted security, container, dependency, and race checks.
- Finish server-backed message delivery/read receipts and improve long-conversation navigation.
- Replace wide assignment selectors with a searchable, accessible user-and-department combobox.
- Exercise upgrade, backup, restore, and rollback procedures against clean and populated databases.
- Keep English and Russian catalogs complete as workflows and validation messages change.

## Next — operational completeness

- Complete edge cases for duplicate merging, CSV error recovery, bulk operations, recurring tasks, and saved views.
- Expand calendar scheduling, reminders, mailbox failure recovery, webhook rotation, and automation diagnostics.
- Improve pipeline forecasting and reporting without introducing unverified or cross-currency aggregates.
- Add documented administrator workflows for roles, stage permissions, departments, and content translation.

## Next — performance and reliability evidence

- Run and publish the 100-VU stretch profile with the bottleneck and resource limits documented.
- Add long-session browser memory scenarios for tables, boards, chat, mail, and calendar.
- Verify optional call behavior with two real browsers, denied permissions, reconnects, and cleanup.
- Repeat bundle, Lighthouse, k6, CPU, RSS, and PostgreSQL query measurements after material changes.

## Version 1.0 criteria

- A clean clone builds and starts through the documented two-container path.
- Supported migrations, upgrades, backup, and restore procedures have reproducible evidence.
- Core sales, collaboration, administration, localization, and tenant-isolation paths pass the required test matrix.
- Security and performance reports describe both passing targets and known misses.
- Public documentation matches the shipped behavior and contains no placeholder results.
- A support and compatibility policy is based on actual release capacity.

Priorities may change when a security issue, data-integrity defect, or reproducible performance regression is found.
