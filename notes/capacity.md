# Capacity — 24h ingest run (P1.06)

**Setup:** `riptide-ingest` v0, single stream `btcusdt@trade`, on the laptop.
Started: 2026-08-25 12:12:22. Stopped: 2026-08-26 02:22:05.

Didn't get 24h. Machine had to go off early, and it slept from 16:47 onward
even with `caffeinate -ims` — waking ~30-55s an hour, enough to reconnect and
grab a burst before dropping again.

So the tape is 14.16h wall but only 4.58h of it is real capture. That first
window holds 97% of the trades and is what everything below measures.
Anything computed over the whole file is diluted by the sleep hours.

## Measured

Window: 12:12:22 → 16:46:58 (4.58 h).

| metric | value |
| --- | --- |
| events (trades) | 1,054,639 |
| avg events/sec | 64.0 |
| peak events/sec (1s bucket) | 3,578 (at 15:21:28) |
| peak 1-min average events/sec | 369.8 |
| bytes/event | 135.5 B |
| bytes/hour | 29.8 MiB/h |
| total tape size | 136.3 MiB (whole file 140.1 MiB) |
| silent seconds (no trades) | 509 of 16,477 (3.09%) |
| reconnects | 0 in window (9 over the whole file, all sleep/wake) |
| missing trade ids | 0 |

## Raw output

`.local/scripts/capacity.sh` reports over the whole file, so don't quote these:

```text
window        2026-08-25 12:12:22 → 02:22:05 (14.16 h wall)
events        1084074 trades
bytes         146853709 (140.1 MiB), 135 B/event
events/sec    avg 21.3  |  peak 3578 (at 15:21:28)  |  busiest minute avg 369.8
bytes/hour    9.9 MiB/h  →  0.23 GiB/day at this rate
silent secs   34683 of 50984 (68.03%)
reconnects    9
```

`avg 21.3`, `silent 68.03%` and `0.23 GiB/day` are all machine sleep, not market.

Per-hour capture, showing where it falls over:

```text
  12:00   228617 trades  3405/3600 live sec
  13:00   210953 trades  3520/3600
  14:00   296448 trades  3506/3600
  15:00   211282 trades  3561/3600
  16:00    61233 trades  1007/3600   ← slept at 16:47
  17:00     4731 trades    34/3600
  18:00    10009 trades    37/3600
  19:00     3502 trades    35/3600
  20:00     2660 trades    56/3600
  21:00     2572 trades    47/3600
  22:00      470 trades    31/3600
  23:00      835 trades    35/3600
  00:00        — nothing
  01:00     4656 trades    58/3600
```

## Notes

- Volume peaked 15:21:28 at 3,578 trades/sec, during a selloff. Price ran
  80,577.75 → 79,130.00 over the window, low 78,888.00. Busiest 10-min bucket
  (15:20) took 104,323 trades, about 3x a normal one. Peak is ~56x the average,
  so sizing off the average is wrong.
- No disconnect cost us anything. Every one of the 9 `disconnected` lines is the
  socket found dead on wake, and each gap closes within seconds of the matching
  `connected` line. Nothing in the clean window.
- Trade ids are monotonic, so the tape audits itself. 1,054,639 ids in the clean
  window, no sequence jumps, nothing missing, no duplicates. Ingest is sound.
  The 9 jumps across the whole file line up exactly with the sleep gaps and
  account for 2,455,472 trades we never saw.
- Those same ids say the market did not slow down overnight: 69.4 trades/sec
  sustained across the full 14.16h span vs 64.0 measured in the clean window.
  The overnight lull in the raw output is entirely our machine.
- `caffeinate -ims` did not hold. The 1/min sampler in `.local/scripts/run-24h.sh`
  fired every ~16 min after 16:47 (308 samples, ~850 due), so the whole process
  tree was suspended, not just the socket. Worth sorting out before any long run.
