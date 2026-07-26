# Distributed Rate Limiter

A horizontally-scalable HTTP rate limiter that enforces **one globally-consistent quota per tenant** across many stateless replicas sharing a single Redis. Built in Go.

The interesting problem isn't rate limiting on one machine — that's a counter in memory. It's keeping that counter **correct and fast** when N copies of the service enforce it concurrently against shared state. This project builds the naive versions first to *demonstrate* how they break, then fixes them with an atomic Redis Lua script, and measures the difference.

> **Status:** early / in progress. The single-node token bucket (Go → Redis → atomic Lua) is implemented and verified. Multi-replica orchestration, alternative algorithms, sharding, and the benchmark suite are on the roadmap below. Benchmark numbers will be published here once measured — none are fabricated.

---

## The core problem

Rate limiting on a single node is trivial. Run three replicas behind a load balancer and two bugs appear:

1. **Split counters** — if each replica keeps its own in-memory counter, a "100 req/s" limit becomes 300 req/s.
2. **The check-then-set race** — move the counter to shared Redis, but read it, decide, and write it back as three separate steps, and two replicas can both read "1 token left", both allow, and both write "0". One token funds two requests.

The fix is to make the entire read → compute → write an **atomic** operation. Redis executes a Lua script as a single indivisible step, which closes the race window without any distributed lock.

---

## Architecture

```
                     ┌────────────────────────────────────┐
   client            │            rate limiter            │
  requests           │                                    │
      │  X-API-Key    │   ┌────────┐                        │
      ▼               │   │  app1  │─┐                       │
 ┌─────────┐          │   └────────┘ │    ┌─────────┐        │
 │  nginx  │──round───┼──▶│  app2  │─┼───▶│  Redis  │  ← single shared
 │  (LB)   │  robin   │   └────────┘ │    │ counter │    source of truth
 └─────────┘          │   ┌────────┐ │    └─────────┘        │
                      │   │  app3  │─┘                       │
                      │   └────────┘                         │
                      │        │ (allowed requests only)     │
                      │        ▼                             │
                      │   ┌──────────┐                       │
                      │   │ upstream │  ← protected backend  │
                      │   └──────────┘                       │
                      └────────────────────────────────────┘
```

- **nginx** distributes requests across replicas round-robin (no sticky sessions — stickiness would mask the correctness bugs this project demonstrates).
- **app replicas** are identical Go processes; each makes its allow/deny decision by calling Redis.
- **Redis** holds the one authoritative bucket per tenant.
- **upstream** is a trivial stub standing in for the real API being protected.

---

## Algorithm: token bucket

A bucket holds up to `capacity` tokens and refills at `rate` tokens/second. Each request spends `cost` tokens; an empty bucket means denied. Two independent knobs — **burst** (capacity) and **sustained rate** (refill) — which is why this is the algorithm Stripe, AWS, and most gateways use.

Refill is **lazy**: rather than a background timer topping up every bucket, the accrued tokens are computed on read:

```
elapsed = max(0, now - last_seen)
tokens  = min(capacity, tokens + elapsed * rate)
```

The whole decision runs inside Redis as one atomic Lua script (`internal/limiter/scripts/token_bucket.lua`). Two deliberate design choices:

- **App-supplied timestamp** (not `redis.call('TIME')`) — keeps the script deterministic and saves a round trip, at the cost of tolerating minor clock skew (handled by the `max(0, …)` guard).
- **Tokens returned as a string** — the Redis Lua→client protocol coerces numbers to integers, which would silently drop fractional tokens; a string preserves them.

---

## Limiter modes

The project implements several limiter backends so their behaviour can be compared directly under identical load. Two are intentionally broken to demonstrate the failure modes above.

| Mode | Description | Expected behaviour |
| --- | --- | --- |
| `memory` | Per-replica in-memory counter, no Redis | Overshoots (~N× the limit) — demonstrates split counters |
| `naive` | Shared Redis, non-atomic read/compute/write | Overshoots under concurrency — demonstrates the race |
| `lua` | Shared Redis, atomic Lua token bucket | Correct |
| `sliding` | Sliding-window log (sorted set) | Correct; different memory/precision trade-off *(planned)* |

---

## Requirements

- Go 1.22+
- Docker (for Redis)

---

## Getting started

Start Redis:

```bash
docker run -d --name rl-redis -p 6379:6379 \
  redis:7-alpine redis-server --save "" --appendonly no
docker exec rl-redis redis-cli PING   # -> PONG
```

Run the single-node token-bucket smoke test (capacity 5, refill 1/s, 7 requests):

```bash
docker exec rl-redis redis-cli FLUSHALL
go run ./cmd/limiter
```

Expected output — the first 5 requests allowed and draining, then denied:

```
request 1 -> allowed=true  remaining=4.0
request 2 -> allowed=true  remaining=3.0
...
request 5 -> allowed=true  remaining=0.0
request 6 -> allowed=false remaining=0.0
request 7 -> allowed=false remaining=0.0
```

Inspect the live bucket in Redis:

```bash
docker exec rl-redis redis-cli HGETALL rl:acme   # tokens + last-seen timestamp
```

Run the upstream stub (the fake protected backend) on port 9000:

```bash
go run ./cmd/upstream
curl localhost:9000/hello   # -> {"ok":true,"path":"/hello"}
```

---

## Project layout

```
cmd/
  limiter/     # the rate-limiter service (smoke test today; HTTP proxy planned)
  upstream/    # trivial backend stub the limiter protects
internal/
  limiter/     # Limiter interface, token bucket, and the Lua script
    scripts/   # token_bucket.lua — the atomic decision
config/        # per-tenant configuration (planned)
nginx/         # load-balancer config for the multi-replica setup (planned)
bench/         # k6 load tests and benchmark harness (planned)
docs/          # design notes
```

---

## Roadmap

| Phase | Deliverable |
| --- | --- |
| 1 ✅ | Single-node token bucket + atomic Lua + broken baselines |
| 2 | 3 replicas + nginx + correctness harness (the headline correctness comparison) |
| 3 | k6 load-test baseline |
| 4 | Sliding-window log + per-tenant algorithm selection |
| 5 | Hot-key sharding to remove single-key contention (throughput headline) |
| 6 | Bounded-concurrency load shedding + connection-pool tuning |
| 7 | Circuit breaker + graceful degradation (fail-open) + Prometheus metrics |
| 8 | Full benchmark sweep + charts |
| 9 | Design docs |

---

## License

MIT
