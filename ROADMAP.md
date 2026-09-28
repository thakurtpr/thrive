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
**Current:** ~60–65% (portable measured; Linux CI authoritative) | **Target:** 70%

- [x] image_test.go — Pull/Mount/Unmount/Push (incl. chunk store, SafeRef)
- [x] runtime_test.go — state roundtrip + lifecycle error paths
- [x] lazypull_test.go — fetch paths, cache hits, HTTP errors
- [x] p2p deeper coverage — DHT ops, peers, chunk request paths
- [x] secrets edge cases — wrong key, corrupt/truncated ciphertext, concurrency
- [x] CLI tests — shared output helpers at ~100%, flag-parity locks, wire-map tests
- [ ] Golden-output tests for images/inspect formatting (biggest remaining lever)
- [ ] `verifyPulledImage` live-registry path (needs mock OCI registry)
- [ ] Live-path tests needing root: OverlayFS mount, veth/iptables, FUSE serve, criu dump

---

## Phase 10: Network Isolation — [x] COMPLETE
**Owner:** NETWORK agent | **Complexity:** HIGH | **Impact:** HIGH

- [x] veth pair creation per container (`internal/network/veth.go`)
- [x] Bridge network with NAT — thrive0 (172.20.0.0/16) + named /16 networks
- [x] CNI plugin interface compatibility (`internal/network/cni.go`)
- [x] DNS resolution inside container (resolv.conf injection)
- [x] Port forwarding: --publish/-p host:container (iptables DNAT + slirp4netns rootless fallback)

---

## Phase 11: Image Signing — [x] COMPLETE
**Owner:** SECURITY agent | **Complexity:** MED | **Impact:** HIGH

- [x] cosign-compatible verification via sigstore-go (`pull --verify --verify-key`, all platforms)
- [x] Signature storage: cosign `.sig` OCI artifacts (read path)
- [x] Verify-on-pull: untrusted images removed on failure
- [x] Key management: thrive-native Ed25519 `sign keygen` / `sign image` / `verify`
- [-] Keyless (Fulcio/Rekor) identities — out of scope (explicit key files only)

---

## Phase 12: systemd Integration — [x] COMPLETE
**Owner:** ORCHESTRATOR agent | **Complexity:** MED | **Impact:** MED

- [x] thrive.socket activation unit + thrive.service (`debian/`, `make install-systemd`)
- [x] Linux daemon entry (`cmd/thrived`)
- [ ] Journal logging integration (sd_journal_send) — stdout JSON is the current sink
- [ ] Cgroup delegation via systemd slice — direct cgroupfs management instead

---

## Phase 13: Rootless Nesting — [-] FUTURE
**Owner:** RUNTIME agent | **Complexity:** VERY HIGH | **Impact:** LOW

- [ ] Containers within containers via nested user namespaces
- [ ] uid_map / gid_map chain for nested rootless
- [ ] Nested OverlayFS stacking
- [ ] Security policy for nesting depth limits
