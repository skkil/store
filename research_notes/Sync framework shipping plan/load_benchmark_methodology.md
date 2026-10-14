# Load-testing and benchmarking methodology for the sync-engine thesis (as of 2026-11-04)

Tags: [F] = backed by a fetched or search-confirmed source. [M] = model knowledge or inference, not verified this session. The shared WebSearch budget ran out partway through (after about 15 searches), so some items that would normally get a citation are tagged [M] or listed under Gaps.

Local context used: `reports/Sync engine thesis novelty check.md` (sections "The server is distributed" and "Recommendation") and `ARCHITECTURE.md` §14 "Concurrency hotspots" (11 hotspots, each listed with its mechanism and what to measure, e.g. "Commit p99 vs devices/owner", "Reclaim lag p99 vs worker count", "Late PUTs accepted after the fence (target 0)").

## 1. Load tools for Go connect-rpc/gRPC + WebSocket + HTTP, offline-fleet simulation, and existing sync-engine harnesses

### Takeaway
Use **k6 with arrival-rate executors** as the main driver (HTTP, gRPC including streaming, WebSockets), **ghz** for focused gRPC microbenchmarks, **vegeta/wrk2** for constant-rate HTTP checks, and **pgbench custom scripts** for the sequencer row. None of the public sync-engine benchmarks (Electric, PowerSync, crdt-benchmarks) simulates offline devices with outboxes and reconnect storms. That harness has to be custom: a Go virtual-client fleet that drives the real client SDK. This is itself a contribution.

### Cited Findings
- k6 implements the open model through the `constant-arrival-rate` and `ramping-arrival-rate` executors. The docs say a closed model lets slow responses lengthen iterations and lower the arrival rate, which "in some testing literature… is known as coordinated omission" [F] — [Grafana k6 docs: open vs closed](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/)
- k6's default `constant-vus` executor is closed-loop. A k6 maintainer comment says avoiding coordinated omission requires independent requests driven by arrival-rate executors [F] — [grafana/k6#2040](https://www.github.com/grafana/k6/issues/2040)
- gRPC streaming support from the old xk6-grpc extension was merged into `k6/net/grpc` in k6 v0.49, and the extension repo is archived [F] — [xk6-grpc](https://github.com/thiagodpf/xk6-grpc); [k6 experimental modules](https://Grafana.com/docs/k6/latest/javascript-api/k6-experimental)
- k6 WebSockets started as xk6-websockets, became `k6/experimental/websockets` in v0.40.0 and implements the browser WebSocket API. Experimental modules may break between versions, and extensions are not supported on k6 Cloud out of the box [F] — [k6 experimental](https://Grafana.com/docs/k6/latest/javascript-api/k6-experimental); [Grafana community](https://community.grafana.com/t/k6-custom-extensions-in-k6-cloud/97373)
- ghz is a gRPC load and benchmark CLI with concurrency, total-request, QPS-limit and duration modes [F] — [ghz (fork mirror)](https://github.com/autonomic-ai/ghz); main repo bojand/ghz, docs at ghz.sh.
- Vegeta targets constant request rates and can export HDR histogram plots. wrk2 is the constant-rate fork of wrk and is used to measure tail latency without coordinated omission [F] — [vegeta](https://www.x-cmd.com/install/vegeta); [wrk/wrk2 guide](https://qaskills.sh/blog/wrk-wrk2-http-benchmarking-guide) (secondary sources)
- **Electric** benchmarks: one Electric server behind a CDN with request collapsing served 100k–1M concurrent users at 960 writes/min. Initial sync was tested with up to 2,000 concurrent clients on a 500-row shape, and live-update latency with up to 10,000 shapes. Write-to-client latency was a flat ~6 ms (~3 ms PG + ~3 ms Electric) for optimised shapes. **Averages only, no p99, no hardware listed.** The docs say results are "highly workload, version and hardware dependent" and that per-release CI benchmarks are planned but not yet in place [F] — [Electric benchmarks](https://electric.ax/docs/reference/benchmarks). The tools are in `electric-sql/client-load-benchmarking` and `electric-sql/1-million-client-benchmark` [F] — [same page](https://electric.ax/docs/sync/reference/benchmarks); [ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Felectric-sql%2F1-million-client-benchmark)
- **PowerSync** publishes limits (>50k concurrent connections per instance, configurable; over the cap new connections get HTTP 429) and a *single-client* Flutter benchmark: 10k–10M rows, server run locally, each metric the **mean of 10 runs**, Linux 100k rows initial sync 4.2 s, 100 incremental updates 39.7 ms, Android 21.8 s / 229 ms. No multi-client load test was found [F] — [PowerSync performance & limits](https://docs.powersync.com/resources/performance-and-limits.md); [PowerSync Flutter benchmark](https://powersync.com/blog/how-fast-is-powersync-performance-benchmarks-for-flutter)
- **crdt-benchmarks (dmonad)**: B4 replays a real character-level editing trace, and B4x100 replays it 100× for a document of >10M chars. Automerge had to run B4 in a single transaction to avoid running out of memory. Hardware: i5-8400, Node 20.5.0. Loro's docs say such benchmarks are "indicators of the absence of performance pitfalls" rather than rankings [F] — [dmonad/crdt-benchmarks](https://github.com/dmonad/crdt-benchmarks); [Loro performance](https://loro.dev/docs/performance)
- The automerge-perf trace is a separate trace of 259,778 single-character ops that produces a 104,852-character document [F] — [search summary of automerge-perf; Eg-walker paper](https://arxiv.org/pdf/2409.14252)

### Inferences
- [M] Locust (Python, closed-model by default, gevent), Gatling (Scala/Java DSL, open and closed injection profiles, good HTML reports) and Tsung (Erlang, XMPP/WebSocket, very high connection counts) all work. k6 fits best because one script covers HTTP, gRPC streaming and WS. Locust and Gatling have no native gRPC support.
- [M] connect-rpc speaks three protocols (Connect over HTTP/1.1 JSON or proto, gRPC, gRPC-Web), so one handler can be loaded by k6 `http` (Connect protocol), k6 `grpc`, and ghz. Report which protocol was used, because their costs differ.
- **Offline-fleet harness (custom, [M]).** k6 VUs are a poor fit for stateful devices that keep a local SQLite, an outbox and a cursor. Instead, write a Go "device simulator" that runs N real client cores in-process (or M processes × N goroutines). Each device follows a scripted lifecycle: online think-time → offline for D ~ distribution (e.g. lognormal, minutes to days of simulated mutations) → accumulates Q outbox entries → reconnects. Reconnect storms are produced by synchronised reconnects (e.g. 10k devices within 1 s after a simulated LB restart) and should be compared with jittered backoff. Drive arrivals open-loop (Poisson schedule, latency measured from the scheduled time), as pgbench does.
- Since no existing sync harness models offline duration and outbox depth, publishing the fleet simulator as a reusable artifact strengthens the "reproducibility" contribution that the novelty report already proposes.

### Gaps
- Zero/Replicache, Ditto and Liveblocks public perf harnesses were not checked (search budget exhausted). Replicache historically shipped a `perf` suite in-repo [M].
- Contents of Electric's 1M-client repo (topology, client implementation) were not read.

## 2. Measurement pitfalls and reporting standards

### Takeaway
Use an open-loop arrival schedule, record latency from the *intended* start time into HdrHistogram, report p50/p99/p99.9 plus max with the sample count, discard warmup, run ≥5–10 independent trials with confidence intervals (or run until a CI target is met), and state the environment fully. Hoefler & Belli SC'15 is the standard to cite. ACM badges are the reproducibility target.

### Cited Findings
- Schroeder, Wierman, Harchol-Balter, "Open Versus Closed: A Cautionary Tale", NSDI '06. Workload generators built for the same purpose use different system models, and often it is not clear which one they use. Closed-model response time depends on the multiprogramming level [F] — [USENIX NSDI'06](https://www.usenix.org/conference/nsdi-06/open-versus-closed-cautionary-tale); [PDF](http://usenix.org/events/nsdi06/tech/full_papers/schroeder/schroeder.pdf)
- Gil Tene's coordinated-omission point: omitted samples that correlate with slow periods skew percentiles. Example: a 10 ms pause with requests due every 50 µs delays ~200 requests, so the true p99 is ~10 ms, not 50 µs. HdrHistogram can correct this when given the expected interval between samples. Tene recommends comparing the corrected and raw histograms, and warns that pauses *outside* the timed window cannot be corrected [F] — [mechanical-sympathy thread](https://groups.google.com/g/mechanical-sympathy/c/icNZJejUHfE); talk "Understanding Latency" [Philly ETE 2014](https://chariotsolutions.com/?p=12688)
- pgbench `--rate` uses a Poisson schedule anchored to the client's start, not the previous transaction's end. The reported latency is "calculated from the scheduled start times", schedule lag is reported separately, and a high lag "is an indication that the system cannot process transactions at the specified rate" [F] — [pgbench docs](https://www.postgresql.org/docs/current/pgbench.html)
- Hoefler & Belli, "Scientific Benchmarking of Parallel Computing Systems: Twelve Ways to Tell the Masses when Reporting Performance Results", SC15 (DOI 10.1145/2807591.2807644). They surveyed 120 papers from top HPC venues and found it was often unclear whether reported improvements were in the noise. The rules cover using speedup with care, not cherry-picking, reporting variability and distributions, comparing data carefully and choosing percentiles. Example wording: "measurements were collected until the 99% CI was within 5% of the mean" [F] — [USI evaluate](https://evaluate.inf.usi.ch/node/366); [Hoefler ISC slides](https://blogs.fau.de/hager/files/2017/02/hoefler-scientific-benchmarking-isc-pe-invited.pdf)
- Benchmarks are workload-, version- and hardware-dependent, and Electric's own page says so [F] — [Electric](https://electric.ax/docs/reference/benchmarks)

### Inferences
- [M] **Warmup and steady state.** Drop the first 1–2 min or the first X% of each run (Go GC, PG buffer cache, connection pools, JIT in k6's goja are all cold). Declare steady state when a rolling-window p50 and throughput stay within ±5% for ≥3 windows. Report the full time series, not only the aggregates.
- [M] **Percentiles.** Report p50/p90/p99/p99.9/max and the number of samples. p99.9 needs at least ~10^4–10^5 samples per configuration to mean anything. Never average percentiles across runs. Merge the HdrHistograms instead, or report each run's percentile with a CI across runs.
- [M] **Trials and statistics.** Use ≥5 (preferably 10) independent runs with a fresh DB per run. Report median plus a 95% bootstrap CI (nonparametric; latency is not normal). Use Mann-Whitney U or a bootstrap difference test when comparing designs. Hoefler & Belli favour nonparametric statistics for skewed data.
- [M] **Environment control.** Run the load generator on a separate host or pinned cores (`taskset`/cgroups cpuset). Disable CPU frequency scaling and turbo (`cpupower frequency-set -g performance`). Keep the machine idle otherwise. Pin `GOMAXPROCS`. Use local NVMe for PG. Record kernel, Go, PG and k6 versions plus `postgresql.conf` diffs. On cloud, use dedicated or bare-metal instances, run interleaved A/B/A/B trials to cancel drift, and report the instance type. Make sure the load generator is never the bottleneck: monitor its CPU, and for k6 watch `dropped_iterations` (dropped iterations mean the VU pool was exhausted, so the target rate was not actually offered).
- [M] **Reporting standards.** ACM artifact badges are Artifacts Available / Evaluated (Functional, Reusable) / Results Reproduced / Replicated. SIGMOD and OSDI/SOSP run artifact-evaluation tracks. Ship the harness, Docker Compose or Nix, seeds and raw HDR logs.

### Gaps
- The exact text of ACM's badge definitions could not be fetched (the acm.org page returned 403). Wording above is [M].
- The full list of Hoefler & Belli's 12 rules was not retrieved verbatim.

## 3. Sync-specific metrics

### Takeaway
Existing vendors report mostly averages for initial sync time, write-to-client latency and memory. A thesis should add distributional metrics for offline/reconnect behaviour (convergence time vs offline duration and queue length, reconnect-storm recovery), per-scope sequencer throughput, fan-out latency, bytes per sync, orphan bytes and reclaim lag, and filesystem metrics (detection latency, scan cost, echo writes, crash-recovery time, hashing throughput).

### Cited Findings
- Electric's metric set: memory use, time to sync all data, write-to-client latency, fan-out average latency, per-change write processing time (0.17–0.27 ms ≈ 4–6k changes/s for optimised shapes) [F] — [Electric](https://electric.ax/docs/reference/benchmarks)
- PowerSync's metric set: initial-sync time and incremental-update latency vs dataset size (10k–10M rows) per platform [F] — [PowerSync Flutter](https://powersync.com/blog/how-fast-is-powersync-performance-benchmarks-for-flutter)
- pgbench supports custom scripts with weights (`-f file@w`), `\set` with `random_zipfian`/`random_exponential`/`random_gaussian` (with `permute()` to break correlation with physical row order), `--max-tries` with `--failures-detailed` for serialization/deadlock retries, and per-transaction or aggregated logs including schedule lag [F] — [pgbench docs](https://www.postgresql.org/docs/current/pgbench.html)
- Syncthing field data: the directory walk is single-threaded. Single-thread SHA-256 hashing ran at ~382 MB/s with `minio/sha256-simd`. inotify can coalesce many changes into a parent-directory event, which forces a rescan. A 380k-file folder scanned in under a minute with tuned DB settings [F, forum anecdotes] — [Syncthing forum: speed up scanning](https://forum.syncthing.net/t/looking-for-ways-to-speed-up-scanning-on-large-folders/12717); [CPU on large folder](https://forum.syncthing.net/t/cpu-usage-on-large-folder/6744); [380k files](https://forum.syncthing.net/t/380k-files-24gb-in-directory-results-in-very-slow-sync-8-days/21488)
- BLAKE3's README says it is "much faster" than SHA-2 and that `b3sum` is "an order of magnitude faster than e.g. sha256sum" on typical desktops. That claim relies on multithreading. The chart uses 16 KiB inputs on a Cascade Lake-SP 8275CL [F] — [BLAKE3 README](https://github.com/BLAKE3-team/BLAKE3)
- The media-service hotspot metrics in the local ARCHITECTURE.md §14 are: commit p99 vs devices/owner, lock wait, DB QPS and cache hit ratio, reclaim lag p99 vs workers, S3 req/s, outbox lag p99, duplicate counts, measured cache staleness distribution, late PUTs after the fence (target 0), batchAccess p99 vs N with DB queries per batch held constant, and pool wait time [F, local] — `/home/hyoungjoojin/Desktop/skkil/ARCHITECTURE.md` §14

### Inferences — proposed metric definitions [M]
- **Convergence time** T_conv(D, Q) = time from a device's reconnect until all its outbox entries are acknowledged *and* every other online replica in the scope reports cursor ≥ the resulting head. Plot it against offline duration D and queue length Q, and fit it (expect roughly linear in Q, with a fixed RTT cost per batch).
- **Bytes per sync**: wire bytes up and down per mutation and per pull, split into payload and overhead. Compute it from proxy counters (Toxiproxy or a tap) rather than app counters.
- **Sequencer throughput**: committed mutations/s per scope at a fixed p99 SLO. Use a pgbench custom script `BEGIN; INSERT effect; UPDATE sync_scope SET head=head+1 … RETURNING; UPDATE client_mut_id …; COMMIT;`, with clients fixed on one scope (hot) vs spread Zipfian over scopes. Ablations: row counter vs `bigserial` vs `pg_advisory_xact_lock` vs SERIALIZABLE+retry. The ceiling should be roughly 1/(commit latency incl. fsync) per scope, so also report results with `synchronous_commit=off` and with group commit (`commit_delay`).
- **Fan-out latency**: time from commit to the poke being received to the pull completing, per subscriber, as a distribution vs subscribers/scope. Use one clock (single host) or a dedicated timestamp echo.
- **Reconnect storm recovery**: time from the mass reconnect until p99 pull latency and error rate return to baseline, plus the peak PG connections and CPU reached.
- **Orphan bytes and reclaim lag**: bytes in storage with no committed reference, integrated over time (byte-seconds), plus the p99 lag from dereference to delete.
- **Filesystem**: change-detection latency (write → event observed, p50/p99), full-scan cost per 10k/100k/1M files (wall time, syscalls, CPU) for cold vs warm caches, **echo writes** (FS writes caused by applying remote changes that are re-detected as local; target 0, which is F5), crash-recovery time vs journal size, hashing GB/s single- and multi-threaded for BLAKE3 vs SHA-256 (SHA-NI) at file sizes 4 KiB–1 GiB.
- **Mobile**: energy via Android Battery Historian / `dumpsys batterystats` or Xcode Energy Log, and radio wake-ups per hour. Secondary for this thesis.

### Gaps
- No primary BLAKE3-vs-SHA-256 single-thread numbers were obtained. Measure them locally; that is better evidence anyway.
- No published "reconnect storm" methodology from a sync vendor was found.

## 4. Correctness under load

### Takeaway
Pair every load experiment with an invariant checker. Use deterministic simulation (seeded, fake clock, fault injection) to show 0 violations per 10^6 schedules and to make the ablations produce violations. Use Jepsen/Elle-style history checks on the real stack under Toxiproxy/netem faults, LazyFS or ALICE-style crash-state exploration for F4, and QuickCheck-style model-based testing (the Dropbox ICST'16 precedent) for F1–F5.

### Cited Findings
- TigerBeetle VOPR runs production code with simulated time, network and storage. Faults are packet drop and reorder, partitions, and disk read/write corruption, and are driven by a seed. A run is reproducible from the seed plus the git commit. 3.3 s of simulation ≈ 39 min of real time, and 10 simulators run 24/7. A 2026 post contrasts its protocol-aware DST with Jepsen-style black-box testing and Antithesis's deterministic hypervisor [F] — [TigerBeetle VOPR docs](https://docs.tigerbeetle.com/about/vopr); [Protocol-aware DST, 2026-08-20](https://tigerbeetle.com/blog/2026-08-20-protocol-aware-dst); [2023 blog](https://tigerbeetle.com/blog/2023-07-11-we-put-a-distributed-database-in-the-browser)
- Go `testing/synctest` was experimental in Go 1.24 and graduated in **Go 1.25 (Aug 2025)**. `synctest.Test(t, f)` replaces `Run`. Inside a bubble, time is fake and `synctest.Wait` blocks until all bubble goroutines are durably blocked. `t.Run`, `t.Parallel` and `t.Deadline` are disallowed inside a bubble [F] — [Applied Go](https://appliedgo.net/spotlight/go-1.25-the-synctest-package/); [Go 1.25 tour](https://antonz.org/go-1-25/); [future-architect](https://future-architect.github.io/articles/20250805a)
- Elle (Kingsbury & Alvaro, PVLDB 14(3), 2021) infers Adya dependency graphs from client-observed histories. It detects every Adya anomaly except predicate ones and was applied to four real databases [F] — [VLDB](https://www.vldb.org/pvldb/volumes/14/paper/Elle%3A%20Inferring%20Isolation%20Anomalies%20from%20Experimental%20Observations); [arXiv 2003.10554](https://arxiv.org/abs/2003.10554)
- Maelstrom is a Jepsen-based workbench. Nodes in any language exchange JSON over stdin/stdout. It simulates latency (`--latency`, `--latency-dist`), message loss and nemeses (`partition`, `kill`, `pause`), and its checkers go up to strict serializability via Elle [F] — [jepsen-io/maelstrom](https://github.com/jepsen-io/maelstrom)
- LazyFS (VLDB 2024) is a FUSE filesystem with its own page cache. On a simulated crash it drops un-fsynced writes. It found 8 new bugs and is integrated in the etcd and MongoDB test setups and in Jepsen. Jepsen's NATS 2.12.1 analysis (Dec 2025) used it to show that acknowledged messages are lost on power failure [F] — [LazyFS VLDB'24](https://www.vldb.org/pvldb/vol17/p3017-ramos.pdf); [Jepsen NATS 2.12.1](https://jepsen.io/analyses/nats-2.12.1)
- Pillai et al., OSDI '14, "All File Systems Are Not Created Equal" (ALICE): 11 applications (LevelDB, SQLite, PostgreSQL, Git, HDFS, ZooKeeper, …), 60 static / 156 dynamic crash vulnerabilities, about half of which manifest on ext3/ext4/btrfs. ALICE explores crash states from recorded syscall traces [F] — [ALICE paper](https://cs.uwaterloo.ca/~alkiswan/papers/alice-osdi14.pdf); [ADSL page](https://research.cs.wisc.edu/adsl/Publications/alice-osdi14.html)
- Hughes, Pierce, Arts, Norell, "Mysteries of Dropbox" (ICST'16) gives the first testable model of a file synchronizer. Its setup was a host plus 3 Ubuntu VMs, QuickCheck in Erlang, and a black-box `STABILIZE` operation that waits for all clients to agree, with **a 30 s cap** before recording "failed stabilization". Random `SLEEP`s (≤1 s) provoke timing bugs, and "triggered" writes that busy-wait on a file change raise the race probability. Shrinking also reduces sleep durations. It found data loss in Dropbox and Google Drive. The authors note that a fixed wait fails because synchronizers back off under rapid change, and that status icons are often wrong [F] — [ICST'16 PDF](https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf)
- Jepsen found G2-item under SERIALIZABLE in PostgreSQL 12.3 (2020) and write loss in Bufstream 0.1.0 (2024) [F, via local novelty report] — [Jepsen PG 12.3](https://jepsen.io/analyses/postgresql-12.3); [Bufstream](https://jepsen.io/analyses/bufstream-0.1.0)

### Inferences
- [M] For Go: run the server instances, client cores and a fake bus/DB inside one process under `synctest` (fake time) or under a custom seeded scheduler. `synctest` gives fake time and quiescence detection, but **not deterministic goroutine interleaving**. For seed-reproducible schedules, interpose your own scheduler at message boundaries (FoundationDB/VOPR style: a single-threaded event loop over simulated nodes). madsim and turmoil are the Rust equivalents and are not directly usable from Go.
- [M] Toxiproxy (latency, bandwidth, timeout, reset_peer, slicer toxics, TCP-level, controllable per test through an API) for client↔server and server↔Redis/NATS/PG. `tc netem` for whole-interface delay, loss and reorder. Chaos Mesh only if running on k8s. Antithesis is a commercial deterministic hypervisor (out of budget for a thesis, but cite it as related work).
- [M] **Check invariants under load, not only in unit tests.** Log every push ack, pull response and cursor to an append-only history. Afterwards check: versions dense per scope, monotonic per client, no cursor skipping a committed version, every acked mutation applied exactly once (D1), and final-state equality across replicas (S1/S4). This is Elle-style reasoning specialised to the log.
- [M] **F4 crash testing**: (i) LazyFS under the client's SQLite plus user folder, killing at random points and dropping un-fsynced data; (ii) ALICE/CrashMonkey-style syscall-trace replay of the folder-apply protocol (write tmp → fsync → rename → fsync dir); (iii) SIGKILL at injected crash points through a failpoint library. CrashMonkey targets kernel filesystem bugs, so ALICE or LazyFS fits better for application-level protocols.

### Gaps
- Current status and APIs of CrashMonkey/B3 and Chaos Mesh were not re-verified.
- ER-π (Middleware 2025) is cited in the local report only and was not re-fetched.

## 5. Baselines and fair comparisons

### Takeaway
Compare against (i) ablations of your own system (the strongest, fairest baselines), (ii) a naive watcher + LWW implementation that you build on the same stack, and (iii) Syncthing for folder sync and PowerSync for server-side sync as external reference points, run on identical hardware with documented configs. Use external systems for qualitative and correctness comparisons (oracle violations) more than for raw-speed shootouts.

### Cited Findings
- Vendors stress that results depend on workload, version and hardware, and recommend testing on your own infrastructure [F] — [Electric](https://electric.ax/docs/reference/benchmarks)
- CRDT benchmark maintainers frame cross-library numbers as "absence of pitfalls" indicators, not rankings [F] — [Loro](https://loro.dev/docs/performance)
- Syncthing's scan and watch behaviour (single-threaded walk, inotify coalescing, tuning knobs like `databaseTuning`, multiple hashers) means its config must be stated in any comparison [F] — [Syncthing forum](https://forum.syncthing.net/t/cpu-usage-on-large-folder/6744)
- The Dropbox/Google Drive QuickCheck study shows that black-box oracle testing of third-party synchronizers is feasible and publishable [F] — [ICST'16](https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf)

### Inferences — fairness rules [M]
- Same machine, same storage, same network shaping (netem profile) and same dataset generator for all systems. Pin versions. Publish configs. Turn off features the other system lacks, or report both settings.
- Compare at **equal offered load** (open model). Report throughput at an SLO (e.g. max mutations/s with p99 < 100 ms), not "max throughput", because max throughput rewards queue blow-up.
- Syncthing is peer-to-peer and has no Postgres. For (a), compare only the *local* half (folder ↔ local index): detection latency, scan cost, echo writes, and crash-safety under LazyFS. Do not compare end-to-end server sync.
- Naive watcher + LWW (fsnotify → upload whole file, last-writer-wins on mtime) is the strawman for F3/F5. It should show lost updates and echo loops under the same oracles.
- "Lightroom-style sidecar" (metadata in XMP files beside images) and Obsidian sync plugins have no public harness. Treat them as design-comparison rows, not measured baselines, unless you script them (Obsidian Git or Remotely Save plugins can be driven through the vault folder).
- PowerSync self-hosted (open-edition service) is the realistic server-side external baseline for (b). Zero and Electric have different write paths (Electric is read-path only), so frame those comparisons around fan-out and read latency.

### Gaps
- Whether PowerSync's open edition license permits publishing benchmark results was not checked. Check any "DeWitt clause" before publishing.

## 6. Recommended evaluation plan per candidate section

### Takeaway
Every section follows the same structure: hypotheses → oracle-checked correctness campaign (simulation + real faults) → open-loop performance sweeps with ablations → external baseline. (a) has the most novel metrics. (b) has the cleanest quantitative story (pgbench + k6). (c) can reuse the §14 hotspot table almost directly.

### Cited Findings
- The local novelty report recommends leading with the folder replica (F4/F5), proposes ablations for D1 (sequencer `bigserial` vs scope-row vs WAL LSN; cache-aside vs log pull; poke loss with vs without periodic pull; Redis vs DB idempotency keys), and proposes reporting violations per 10^6 seeded schedules, convergence vs offline duration, bytes/sync, p99 local write latency and orphan-byte persistence [F, local] — `/home/hyoungjoojin/Desktop/skkil/store/reports/Sync engine thesis novelty check.md`
- §14 of ARCHITECTURE.md lists 11 media-service hotspots with mechanisms and measures [F, local] — `/home/hyoungjoojin/Desktop/skkil/ARCHITECTURE.md`

### Inferences — the plan [M]

**(a) Filesystem-as-replica (folder ↔ SQLite ↔ Postgres; F1–F5)**
| # | Hypothesis | Experiment | Metrics | Tools |
|---|---|---|---|---|
| A1 | F1–F5 hold under arbitrary interleavings | Model-based PBT (state machine of folder ops + remote ops + crash + STABILIZE), ≥10^6 seeded schedules, plus a naive watcher+LWW baseline | violations per 10^6 schedules per law, shrunk counterexamples | Go `rapid`/`gopter` (QuickCheck-style), seeded sim, `synctest` for time |
| A2 | F4: no acked write lost, no torn file after power loss | Crash at every syscall boundary of the apply/ingest protocol, and random kill under load | crash states explored, violations, recovery time vs journal size | LazyFS (FUSE), ALICE-style trace replay, failpoints, SIGKILL |
| A3 | F5: applying a remote change causes 0 re-uploads | Burst of N remote changes, count local-origin mutations within a window, then check quiescence | echo writes (target 0), time to quiescence | event log + oracle; naive baseline shows echo loops |
| A4 | Change-detection p99 < X ms up to 100k files | Open-loop file-write workload (Poisson) at 1–1000 ops/s | detection latency p50/p99/p99.9, missed events, rescans triggered | fsnotify/inotify vs periodic scan; HdrHistogram |
| A5 | Scan cost is linear and dominated by stat, not hash, with an mtime/size/inode cache | Cold and warm full scan at 10k/100k/1M files and various size distributions | wall, CPU, syscalls, bytes hashed | `perf stat`, `strace -c`, drop_caches |
| A6 | BLAKE3 beats SHA-256 for large files; SHA-NI narrows the gap for small files | Hash throughput 4 KiB–1 GiB, 1 vs N threads | GB/s | Go `lukechampine.com/blake3` / `zeebo/blake3` vs `crypto/sha256` |
| A7 | Convergence grows ~linearly with offline queue length | 2–10 devices on one folder, offline D, queue Q, then reconnect | T_conv(D,Q), bytes/sync, conflict files created | device simulator + Toxiproxy |
| A8 | Compared with Syncthing (local half) | same A4/A5/A2 workloads | same metrics | Syncthing at a pinned version, stated config |

**(b) Server side (per-scope sequencer, stateless instances, poke-then-pull, D1)**
| # | Hypothesis | Experiment | Metrics | Tools |
|---|---|---|---|---|
| B1 | The scope-row sequencer sustains ≥K commits/s per hot scope and scales linearly across scopes | pgbench custom script, hot single scope vs Zipfian over 10^2–10^5 scopes, `-R` sweep, `--max-tries` | TPS at p99 SLO, lock wait (`pg_stat_activity`, `pg_locks`), schedule lag | pgbench, `pg_stat_statements` |
| B2 | Ablation: `bigserial` cursors skip rows; advisory lock ≈ row lock; SERIALIZABLE retries cost more | Same as B1 plus a cursor-skip checker | skipped versions (should be >0 for bigserial), throughput | pgbench + history checker |
| B3 | Push/pull throughput scales linearly with stateless instances until PG saturates | k6 `ramping-arrival-rate` over connect-rpc, 1/2/4/8 instances | throughput at SLO, p99, PG CPU | k6, ghz, Prometheus |
| B4 | Fan-out p99 grows sub-linearly with subscribers per scope | N subscribers via WS/stream; writer at fixed rate | commit→poke→pull-complete latency distribution | k6 WS + gRPC stream, Redis vs NATS |
| B5 | With periodic/gap pull, poke loss causes bounded staleness, not divergence | Toxiproxy drops X% of bus messages | staleness distribution, divergence count (target 0 with pull, >0 without) | Toxiproxy, checker |
| B6 | Reconnect storm recovers in < T s with jittered backoff | 10k–100k simulated devices reconnect within 1 s, after an outbox of Q | recovery time, peak PG connections, error rate, server CPU | device simulator (Go), pgbouncer |
| B7 | D1: idempotency holds under retries, instance kills and failover | Kill instances mid-push, duplicate requests, Redis-held vs DB-held keys | duplicate applications (target 0 with DB keys) | Maelstrom-like harness or a custom Jepsen-lite in Go, Elle-style checker |
| B8 | Simulated topology (multi-instance, replica lag, epoch bump) | Seeded DST with ≥10^6 schedules | violations per 10^6 per ablation | custom Go simulator + `synctest` |

**(c) Blob integrity and rejection (S1/S2/S5 with the media service)**
| # | Hypothesis | Experiment | Metrics | Tools |
|---|---|---|---|---|
| C1 | No orphan persists past the retention window (S5) | Uploads with random abandon, reject or crash; run the reclaimer | orphan byte-seconds, reclaim lag p99 vs workers | §14 #3, checker |
| C2 | A rejection cascade removes all dependents across services (S2) | Rejected inspection with cross-user annotations; saga with injected failures | cascade completion time, residual dependents (target 0) | Toxiproxy between services, Kafka/outbox lag |
| C3 | Fence: 0 late PUTs accepted | Race late PUTs against the fence | accepted late PUTs (target 0), extra requests per cancel | §14 #9, MinIO + Toxiproxy |
| C4 | Commit p99 is stable as devices/owner grow | `FOR UPDATE` vs optimistic version | commit p99, lock wait, I5 violations | §14 #1, k6 |
| C5 | batchAccess makes a constant number of DB queries | N = 10–1000 URLs per request | p99 vs N, DB queries per batch | §14 #10, k6 |
| C6 | Baseline vs PowerSync-attachments-style queue and B0 blog flow | Same fault schedule | orphan count and bytes, violations | per local novelty report |

**Common protocol for all sections [M].** Fix the hardware and record its specs. Run each configuration ≥5–10 times on a fresh DB. Discard warmup. Use open-loop arrivals. Use HdrHistogram with intended-start timing. Report p50/p99/p99.9/max with sample counts, median plus bootstrap 95% CI, and full time series. Publish the harness, seeds, raw logs and Docker Compose, targeting ACM "Available + Functional" badges.

**Pick (my inference).** (b) gives the most defensible load and benchmark story (standard tools, crisp hypotheses, ablations that generate violations). (a) gives the most novel metrics but needs more custom tooling (LazyFS, PBT, a scan harness). (c) is closest to already being planned (§14 + §16).

### Gaps
- No specific target numbers (K, X, T) are justified yet. Set them from a pilot run.
- The Go PBT library choice (`pgregory.net/rapid` vs `gopter`) was not compared this session.
