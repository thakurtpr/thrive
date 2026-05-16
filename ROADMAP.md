# THRIVE Roadmap

Status legend: `[x]` complete · `[~]` in progress · `[ ]` pending · `[-]` future/deferred

---

## Phase 1: Core Runtime — [x] COMPLETE
**Owner:** RUNTIME agent | **Complexity:** HIGH | **Impact:** CRITICAL

- [x] clone(2) with CLONE_NEWNS, CLONE_NEWPID, CLONE_NEWUTS, CLONE_NEWIPC, CLONE_NEWNET
- [x] User namespace mapping (rootless UID/GID map)
- [x] cgroup v2 hierarchy under `/sys/fs/cgroup/thrive/{id}/`
- [x] PID limit, memory limit, CPU quota via cgroup v2 controllers
- [x] Container state machine: created → running → stopped
- [x] State persistence via `state.json` at `/run/thrive/containers/{id}/`

---

## Phase 2: Image Management — [x] COMPLETE
**Owner:** IMAGE agent | **Complexity:** HIGH | **Impact:** CRITICAL

- [x] OCI image pull via go-containerregistry (manifest fetch + layer download)
- [x] Layer extraction: compressed tar → `/var/lib/thrive/images/{ref}/layers/{digest}/`
- [x] Content-addressed chunk store: SHA-256 chunks at `/var/lib/thrive/chunks/{xx}/{rest}`
- [x] OverlayFS assembly: lowerdir (reversed layers) + upperdir + workdir + mergeddir per container
- [x] fuse-overlayfs fallback for kernels < 5.11 / non-root environments
- [x] Image push: re-tar extracted dirs → tarball.LayerFromReader → remote.Write

---

## Phase 3: CLI — [x] COMPLETE
**Owner:** ORCHESTRATOR agent | **Complexity:** MED | **Impact:** HIGH

- [x] `thrive run` — flags: --detach/-d, --rm, --name, --env/-e, --secret
- [x] `thrive ps` — list running/stopped containers
- [x] `thrive kill` — send signal to container process
- [x] `thrive logs` — stream log file with --follow/-f
- [x] `thrive rm` — remove stopped container state
- [x] `thrive images` — list local images
- [x] `thrive pull` / `thrive push` — real registry operations
- [x] `thrive build` — Thrivefile-driven DAG build
- [x] `thrive secret` — encrypt/decrypt/list secrets
- [x] `thrive metrics` — expose Prometheus metrics endpoint
- [x] `thrive system` — system info and resource usage

---

## Phase 4: Thrivefile + DAG Build Engine — [x] COMPLETE
**Owner:** BUILD agent | **Complexity:** HIGH | **Impact:** HIGH

- [x] Thrivefile YAML parser (FROM, RUN, COPY, ENV, EXPOSE, CMD directives)
- [x] DAG topological sort with cycle detection
- [x] Parallel step execution respecting dependency edges
- [x] Real container-per-step execution with poll-until-stopped
- [x] Build cache keying by step content hash (SHA-256)

---

## Phase 5: Secrets Manager — [x] COMPLETE
**Owner:** SECRETS agent | **Complexity:** MED | **Impact:** HIGH

- [x] AES-256-GCM encrypt/decrypt for secret values
- [x] Auto-generate master key on first run, persist at `/var/lib/thrive/secrets/.master` (chmod 0600)
- [x] tmpfs mount inside container to expose secrets (never via env vars)
- [x] Secret listing and removal CLI subcommands

---

## Phase 6: Lazy Pulling via FUSE — [x] COMPLETE
**Owner:** LAZYPULL agent | **Complexity:** HIGH | **Impact:** MED

- [x] FUSE filesystem skeleton with go-fuse
- [x] On-demand chunk fetch via HTTP GET to OCI registry blob endpoint
- [x] Chunk cache on local disk; served from cache on subsequent reads
- [x] Integration with chunk store addressing scheme

---

## Phase 7: OTEL Observability — [x] COMPLETE
**Owner:** OTEL agent | **Complexity:** MED | **Impact:** MED

- [x] Prometheus metrics: container start/stop counters, image pull duration, cgroup stats
- [x] OTLP gRPC trace exporter wired when `OTEL_EXPORTER_OTLP_ENDPOINT` is set
- [x] Span propagation across CLI → runtime → image subsystems

---

## Phase 8: P2P Registry (Kademlia DHT) — [x] COMPLETE
**Owner:** P2P agent | **Complexity:** HIGH | **Impact:** MED

- [x] Kademlia DHT for chunk location discovery
- [x] BitTorrent-style parallel chunk fetch from peers
- [x] Peer add/remove/select with routing table management
- [x] RequestChunk with blocking 30s timeout
- [x] Bootstrap node support

---

## Phase 9: Test Coverage to 70%+ — [~] IN PROGRESS
**Owner:** ORCHESTRATOR agent | **Complexity:** MED | **Impact:** HIGH
**Current:** ~25% | **Target:** 70%

- [ ] image_test.go — Pull/Mount/Unmount/Push integration tests
- [ ] runtime_test.go — expand exec/kill/logs coverage
- [ ] lazypull_test.go — FUSE chunk fetch mocking
- [ ] p2p deeper coverage — bootstrap, routing table eviction
- [ ] secrets additional edge cases — wrong key, corrupt ciphertext
- [ ] CLI integration tests — golden output comparison

---

## Phase 10: Network Isolation (CNI) — [ ] PENDING
**Owner:** NETWORK agent (TBD) | **Complexity:** HIGH | **Impact:** HIGH

- [ ] veth pair creation per container
- [ ] Bridge network with NAT (iptables MASQUERADE)
- [ ] CNI plugin interface compatibility
- [ ] DNS resolution inside container (resolv.conf injection)
- [ ] Port forwarding: --publish/-p host:container

---

## Phase 11: Image Signing — [ ] PENDING
**Owner:** SECURITY agent (TBD) | **Complexity:** MED | **Impact:** HIGH

- [ ] cosign-compatible signature generation and verification
- [ ] Signature storage in OCI registry (`.sig` suffix tag)
- [ ] Verify-on-pull policy enforcement
- [ ] Key management CLI subcommands

---

## Phase 12: systemd Integration — [ ] PENDING
**Owner:** ORCHESTRATOR agent | **Complexity:** MED | **Impact:** MED

- [ ] thrive.socket activation unit
- [ ] thrive@.service template for named containers
- [ ] Journal logging integration (sd_journal_send)
- [ ] Cgroup delegation via systemd slice

---

## Phase 13: Rootless Nesting — [-] FUTURE
**Owner:** RUNTIME agent | **Complexity:** VERY HIGH | **Impact:** LOW

- [ ] Containers within containers via nested user namespaces
- [ ] uid_map / gid_map chain for nested rootless
- [ ] Nested OverlayFS stacking
- [ ] Security policy for nesting depth limits
