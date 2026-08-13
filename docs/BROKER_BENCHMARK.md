# Kafka and RabbitMQ benchmark protocol

Status: dual-broker delivery and a real CRM outbox path were tested on 2026-08-13; comparative performance not measured.

## Question

Does the optional broker profile increase asynchronous throughput or reduce PostgreSQL work enough to justify its infrastructure cost without materially degrading API latency?

## Matrix

Run each scenario on the same commit, machine, dataset, container runtime, network, limits, and warm-up procedure:

1. PostgreSQL outbox/jobs only;
2. Kafka publication enabled;
3. RabbitMQ publication enabled;
4. both brokers enabled.

For each mode run steady load for 10-15 minutes and a five-times burst at 50, 100, and 250 virtual users. Include a single hot workspace, many workspaces, a slow webhook, slow SMTP, broker restart, and recovery drain.

## Record

- API throughput, errors, p50/p95/p99, and request bytes;
- outbox commit latency and age of the oldest unpublished event;
- publish-confirm p50/p95/p99 and end-to-end event latency;
- Kafka producer errors, consumer lag, partition skew, and rebalances;
- RabbitMQ ready, unacknowledged, redelivered, and dead-letter counts;
- duplicate delivery count and duplicate side-effect count;
- backlog drain time after outage;
- application, PostgreSQL, Kafka, and RabbitMQ CPU/RSS;
- PostgreSQL connections, WAL, lock time, and query plans;
- network and disk bytes for every container.

## Acceptance gate

The profile is beneficial only when all of the following hold:

- zero lost events and zero duplicate side effects;
- API error rate below 0.5%;
- API p95 regression no greater than 5%;
- at least twice the asynchronous event/command throughput **or** at least 30% less PostgreSQL CPU/WAL at the same accepted API load;
- recovered backlog drained within twice the burst duration;
- no OOM, goroutine leak, unbounded queue, or database pool starvation;
- broker resource and operational costs included in the report.

Until this matrix has completed successfully, README and performance documentation must show broker performance as **Not measured**.
