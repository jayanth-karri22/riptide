# Capacity model

How much data a Riptide deployment ingests and stores, measured against Binance production
streams, and what that implies for the cost of running the hosted service.

All figures are measured unless explicitly marked as extrapolated. Run detail for the
single-stream baseline is in the appendix.

## Summary

| | |
| --- | --- |
| Sustained ingest, 10 pairs | ~700 messages/sec |
| Burst design target | ~20,000 messages/sec |
| Stored, after envelope strip and compression | 5.5 bytes/message |
| Archive, 10 pairs, one exchange | **~110 GiB/year** |
| Archive, 10 pairs, three exchanges (extrapolated) | **~336 GiB/year** |

Storage is not the cost driver at this scale. Compute, egress and model calls are.

## Method

Two capture runs against `stream.binance.com`.

| run | streams | clean window | messages |
| --- | --- | --- | --- |
| 2026-08-25 | `btcusdt@trade` | 4.58 h, 06:42–11:17 UTC | 1,054,639 |
| 2026-08-26 | 10 pairs × {`@trade`, `@bookTicker`} | 19.6 min, 04:20–04:40 UTC | 439,839 |

Pairs span three liquidity tiers so that pair-scaling is measured rather than assumed: btcusdt,
ethusdt (tier 1); solusdt, xrpusdt, bnbusdt, dogeusdt (tier 2); adausdt, avaxusdt, linkusdt,
ltcusdt (tier 3).

`@bookTicker` payloads carry no timestamp, so the sampler records a local receive clock in a
sidecar alongside the raw tape, leaving the tape byte-honest. Both runs recorded zero dropped
seconds and zero reconnects inside the windows quoted above.

## Measured rates

Rates are for the 19.6-minute window and are corrected for time of day in the next section. The
ratio column is the load-bearing one.

| pair | trade | bookTicker | total | msg/s | × btcusdt |
| --- | --- | --- | --- | --- | --- |
| btcusdt | 43,701 | 108,174 | 151,875 | 129.4 | 1.000 |
| ethusdt | 18,867 | 48,092 | 66,959 | 57.0 | 0.441 |
| xrpusdt | 14,171 | 44,904 | 59,075 | 50.3 | 0.389 |
| dogeusdt | 5,389 | 45,139 | 50,528 | 43.0 | 0.333 |
| solusdt | 8,756 | 24,509 | 33,265 | 28.3 | 0.219 |
| bnbusdt | 6,617 | 20,658 | 27,275 | 23.2 | 0.180 |
| ltcusdt | 3,897 | 12,340 | 16,237 | 13.8 | 0.107 |
| linkusdt | 2,875 | 12,976 | 15,851 | 13.5 | 0.104 |
| adausdt | 1,448 | 10,358 | 11,806 | 10.1 | 0.078 |
| avaxusdt | 814 | 6,154 | 6,968 | 5.9 | 0.046 |
| **total** | **106,535** | **333,304** | **439,839** | **374.6** | **2.897** |

### Findings

**Ten pairs cost 2.90× btcusdt, not 10×.** Liquidity follows a power law and btcusdt sits at the
top of it: avaxusdt is 4.6% of btcusdt, and the bottom four pairs together are under 34% of it.
Sizing an archive as ten times a single measured pair overstates it by roughly 3.4×.

**bookTicker dominates trade volume, 3.13 : 1 by message count.** Quotes move without trades, so
book updates are not bounded by trade count. bookTicker accounts for 75.8% of messages and 71.4%
of payload bytes.

**Per-message cost runs the other way.** bookTicker payloads average 106.1 B against 133.1 B for a
trade, so the byte ratio (2.49 : 1) is gentler than the message ratio. The two size different
things: message rate sizes the broker and sinks, byte volume sizes the archive.

## Wire bytes to stored bytes

What arrives is not what is kept. Three reductions apply, all measured against the captured tape.

| stage | bytes | vs wire |
| --- | --- | --- |
| Wire, as captured | 66,668,815 | 1.00× |
| Payload only, multiplexing envelope removed | 49,974,422 | 1.33× |
| Payload, `zstd -19` | 2,407,013 | **27.7×** |

The envelope is an artifact of multiplexing 20 streams over one connection and accounts for 25.7%
of wire bytes. It carries nothing a sink retains — the symbol is already present in every payload.

Compression is measured with `zstd -19` over envelope-stripped JSONL. This is a conservative proxy
for the intended storage path: the archive lands in Iceberg as Parquet, and columnar encoding of
repeated symbols, monotonic identifiers and prefix-sharing prices should outperform row-oriented
JSONL, not underperform it.

Net: **5.5 bytes stored per message.**

## Time-of-day correction

The multi-stream window sits at 04:20–04:40 UTC, near the daily low for crypto flow. The
single-stream run covered 06:42–11:17 UTC and included a selloff. The quiet window understates a
typical hour and is not quoted raw.

The correction is measured: the multi-stream run re-captured `btcusdt@trade` alongside the other
19 streams, giving a direct ratio against two independent baselines from the earlier run.

| baseline | earlier run | this run | scalar *k* |
| --- | --- | --- | --- |
| Clean-window average | 64.0 msg/s | 37.2 msg/s | 1.72 |
| 14.16 h span, via trade-id deltas | 69.4 msg/s | 37.2 msg/s | 1.86 |

Both are carried forward as a band rather than collapsed to a point estimate.

The pair-ratio sum measured 2.93× after 79 seconds and 2.90× after 19.6 minutes and 440k messages;
per-stream byte sizes are stable to a tenth of a byte. The sample therefore fixes the *shape* of
the workload precisely and *bounds* its magnitude. The limitation is which hour was sampled, not
how much of it — which is what *k* corrects.

## Storage projection

Ten pairs, trade and bookTicker, one exchange.

| | msg/s | wire GiB/day | stored GiB/day | stored GiB/year |
| --- | --- | --- | --- | --- |
| As measured (quiet) | 375 | 4.57 | 0.165 | 60.3 |
| *k* = 1.72 | 644 | 7.86 | 0.284 | 103.6 |
| *k* = 1.86 | 697 | 8.50 | 0.307 | 112.0 |

Assuming comparable per-exchange volume — extrapolated, not measured — three exchanges gives
**0.92 GiB/day and ~336 GiB/year**.

A complete, queryable, twelve-month multi-exchange archive is roughly a third of a terabyte and
fits on a single commodity disk. This bounds the storage component of the cost base.

## Throughput

Storage averages out; throughput does not, and is sized from separate evidence.

| | value | source |
| --- | --- | --- |
| Sustained, quiet window | 375 msg/s | measured |
| Sustained, *k*-corrected | 644–697 msg/s | derived |
| Aggregate 1 s peak | 6,060 msg/s (16.2× window average) | measured |
| Single-stream peak / average | 56× (3,578 msg/s) | measured, during a selloff |

The 6,060 msg/s aggregate peak occurred inside a quiet window. The 56× multiple was measured on a
single stream under real market stress, which the multi-stream window did not contain. Peaks across
pairs are correlated, since a market-wide move spikes every pair simultaneously, so the aggregate
multiple under stress should sit nearer 56× than 16.2×.

**Design target: sustain ~700 msg/s, absorb bursts to ~20,000 msg/s without loss.**

This is the requirement behind placing a durable log between ingest and storage rather than writing
directly to Postgres. At 20,000 msg/s, a few seconds of sink backpressure is a data-loss event.

## Product thesis

**Wedge.** Deep, queryable, time-travelable history with an AI interface that accepts plain
language. The competing tier is strong at *live* and weak at *past*. Riptide keeps every day of
tape in an open table format, exposed through SQL and through an agent that writes the SQL.
Questions of the form "what did funding do in the hour before every 5% drawdown last quarter" are
the target.

**Cost structure.** No price is set yet. What the projection above establishes is the shape of the
cost base: a full single-exchange archive is ~110 GiB/year, and three exchanges ~336 GiB, which is
marginal against the cost of the cluster itself. The expensive components are model calls, metered
per key and capped per tenant, and fixed infrastructure that amortises across subscribers. Each
additional subscriber adds query load and model spend but almost no storage, since they read the
same archive. That is what makes a price far below the incumbent tier plausible; the figure will be
set once hosted infrastructure cost is measured rather than estimated.

**Open core.** The product is Apache-2.0 and self-hostable at no cost; the hosted convenience and
the accumulated archive are the business. Self-hosting is a real offer, but it starts with an empty
archive, and historical tape cannot be backfilled retroactively at any price. That asymmetry is the
durable advantage, and the figures above are what make accumulating it inexpensive.

**Scope.** The first version targets live tape, historical depth, watchlists and alerts, and the
AI interface. What comes after that is undecided rather than ruled out.

## Limitations

- One 19.6-minute window, one day, one exchange, a weekday, a quiet hour, no volatility event. The
  shape is stable and converged; the magnitude is bounded, not pinned.
- The three-exchange multiplier is assumed. Other venues differ in message format and update
  cadence. This is the weakest figure here and should be replaced with a measurement before being
  relied on.
- Parquet compression is proxied by JSONL + zstd. Expected to be pessimistic, but unverified until
  the Iceberg sink exists.
- No retention policy is modelled. Every message is assumed kept indefinitely at full fidelity.
  Downsampling historical book data would reduce the archive substantially and has not been costed.
- Hosted infrastructure cost is not yet quantified. This document argues storage is not the cost
  driver; it does not establish the total.
- Weekday and monthly seasonality are unmeasured. *k* corrects for time of day only.

A longer multi-pair run on dedicated infrastructure closes most of these and supersedes the *k*
correction with direct measurement.

## Appendix — single-stream baseline run

The baseline run captured `btcusdt@trade` alone, and supplies the *k* correction and the 56× peak
multiple used above. Times are UTC.

Started 2026-08-25 06:42:22, stopped 2026-08-25 20:52:05. The machine suspended partway through, so
the tape spans 14.16 h of wall clock but only 4.58 h of continuous capture. That first window holds
97% of the trades and is the only part quoted; anything computed across the whole file is diluted by
the suspended hours.

Window: 06:42:22 → 11:16:58 UTC (4.58 h).

| metric | value |
| --- | --- |
| Events (trades) | 1,054,639 |
| Average events/sec | 64.0 |
| Peak events/sec, 1 s bucket | 3,578 (at 09:51:28) |
| Peak 1-minute average events/sec | 369.8 |
| Bytes/event | 135.5 B |
| Bytes/hour | 29.8 MiB/h |
| Tape size, window | 136.3 MiB (whole file 140.1 MiB) |
| Silent seconds | 509 of 16,477 (3.09%) |
| Reconnects | 0 in window (9 across the whole file, all suspend/wake) |
| Missing trade ids | 0 |

Volume peaked at 09:51:28 during a selloff: price ran 80,577.75 → 79,130.00 across the window, low
78,888.00, and the busiest 10-minute bucket took 104,323 trades — roughly 3× a normal one. Peak was
~56× the average, which is why throughput is not sized from average rate.

Trade ids are monotonic, so the tape audits itself. All 1,054,639 ids in the window are contiguous:
no sequence jumps, no duplicates. The 9 jumps across the whole file align exactly with the suspend
gaps and account for 2,455,472 trades not captured.
