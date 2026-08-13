# ADR 0021: Optional Kafka and RabbitMQ high-throughput profile

Date: 2026-08-13

Status: accepted, performance benefit not yet measured

## Context

The PostgreSQL outbox and job queue keep the base deployment small and provide an atomic boundary with CRM mutations. At substantially higher asynchronous event rates, projection fan-out, queue scans, and slow external work can compete with request traffic for PostgreSQL CPU, WAL, locks, and connections.

A broker cannot improve synchronous SQL latency and does not justify increasing the operational floor for small installations. Kafka and RabbitMQ solve different problems and should not be treated as interchangeable dependencies.

## Decision

The first implemented increment is the durable outbox-to-broker publication boundary and a real confirmation smoke against both brokers. The consumer topology below is the accepted target architecture; consumers are not implemented yet and must pass the fault and comparative load gates before production enablement.

Keep PostgreSQL as the system of record, transactional outbox, retry scheduler, idempotency ledger, and SSE replay store. Add an optional `high-throughput` profile:

- Kafka carries ordered, replayable domain-event pointers for independently scalable projections. Records use the fixed `veltrix.domain-events.v1` topic and are keyed by `workspace_id/aggregate_type/aggregate_id`.
- RabbitMQ carries persistent command/event pointers for low-latency work distribution. The local profile uses a durable topic exchange, a quorum queue, mandatory routing, persistent messages, and publisher confirms.
- Request handlers never write directly to either broker. The committed outbox fans out idempotent publish jobs. Broker failure therefore creates backlog instead of failing the CRM mutation.
- Delivery is at least once. Stable event IDs and consumer idempotency absorb ambiguous confirmation and redelivery. No end-to-end exactly-once claim is made.
- Messages contain tenant and aggregate identifiers, not contact, note, message, or credential content. Consumers must validate the envelope, establish transaction-local workspace context, and reload canonical state under RLS.
- Browser SSE replay remains in PostgreSQL. Kafka consumer groups are not used for cross-replica browser broadcast.

The local Compose services are single-node integration fixtures. A production topology requires Kafka replication, RabbitMQ quorum members, TLS, least-privilege identities, ACLs, monitoring, and independent failure-domain design.

## Where each broker helps

| Workload                                                    | Transport  | Reason                                                                 |
| ----------------------------------------------------------- | ---------- | ---------------------------------------------------------------------- |
| Search/reporting projections and downstream event consumers | Kafka      | Partition ordering, replay, consumer groups, high-throughput batching  |
| Webhook/email/automation/import/media commands              | RabbitMQ   | Work distribution, prefetch, routing, confirms, manual acknowledgement |
| CRM writes, idempotency, retry schedule, audit, SSE replay  | PostgreSQL | Atomic relational state and tenant RLS                                 |

## Reliability and security rules

- Kafka uses all-ISR acknowledgements and an idempotent bounded producer. RabbitMQ uses confirms and mandatory routing.
- Production requires verified TLS and authenticated least-privilege clients; plaintext local settings are rejected by production configuration.
- Broker names and routing keys are deployment-owned static configuration, never tenant input.
- Payloads are bounded to 256 KiB and parsed with a strict versioned JSON schema.
- A publish confirmation followed by process failure can create a duplicate. Consumers must persist a consumer/event inbox or an equivalent unique idempotency key with the effect.
- Poison messages have bounded retry and a confirmed dead-letter path before source acknowledgement.
- Broker credentials and payloads must not appear in logs, error responses, or metric labels.

## Consequences

The base profile remains one application plus PostgreSQL. The high-throughput profile adds substantial memory, CPU, network, storage, monitoring, upgrade, and incident-response cost. It is appropriate only after workload measurements show PostgreSQL queue contention or downstream fan-out pressure.

No performance improvement is claimed yet. Adoption requires a controlled PostgreSQL-only versus Kafka/RabbitMQ comparison under identical data, request load, and resource limits. See `docs/BROKER_BENCHMARK.md`.
