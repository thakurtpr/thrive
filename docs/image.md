# Image Subsystem — Deep Dive

Package: `internal/image`
Owner: IMAGE agent
See also: ARCHITECTURE.md, DECISIONS.md ADR-004, ADR-005, RISKS.md RISK-006

---

## Responsibilities

1. **Pull** — fetch OCI image layers from a registry and extract them to disk
2. **Mount** — assemble extracted layers into a single OverlayFS rootfs for a container
3. **Unmount** — detach the OverlayFS when a container exits
4. **Push** — re-package extracted layer directories and upload to a registry
5. **ChunkStore** — content-addressed storage for chunk-level deduplication and P2P transfer

---

## OCI Image Pull Flow

```
name.ParseReference(ref)
  -- go-containerregistry parses "alpine:3.19" -> registry + repo + tag

remote.Get(parsed, remote.WithAuth(authn.Anonymous))
  -- fetches manifest JSON from registry API

descriptor.Image()
  -- resolves to a v1.Image (OCI Image Spec compliant)

img.Layers()
  -- returns []v1.Layer in order (bottom layer first)

for each layer:
  layer.Digest()         -> digestHex (e.g. "sha256:abcdef...")
  check .done marker     -> skip if already fully extracted (idempotent pull)
  layer.Uncompressed()   -> io.ReadCloser (raw tar stream, decompressed)
  extractTar(rc, destDir)
    where destDir = /var/lib/thrive/images/{ref}/layers/{digest}/
  write .done marker     -> marks extraction complete

json.Marshal(manifest{Ref, Digest, Layers[]})
  -> /var/lib/thrive/images/{ref}/manifest.json
```

Authentication: `authn.Anonymous` for public images. For private registries, `authn.Basic`
with credentials passed via CLI flags `--username` / `--password`, or read from
`~/.docker/config.json` via go-containerregistry's default keychain.

---

## Layer Extraction: extractTar

`extractTar(rc io.Reader, destDir string)` unpacks an uncompressed tar stream:

| Entry Type | Action |
|------------|--------|
| `TypeDir` | `os.MkdirAll(path, mode)` |
| `TypeReg` / `TypeRegA` | Create file with `O_CREATE\|O_TRUNC\|O_WRONLY`; copy body |
| `TypeSymlink` | `os.Remove` existing + `os.Symlink(link, path)` |
| `TypeLink` (hardlink) | `os.Link(target, path)` |
| `.wh.` prefix (whiteout) | NOT YET HANDLED — see RISKS.md RISK-006 |

Path traversal protection: every header name is cleaned with `filepath.Clean` and checked
that the result does not escape `destDir`. Entries attempting to write outside `destDir`
are skipped.

---

## Disk Layout After Pull

```
/var/lib/thrive/images/
  index.docker.io/library/alpine:3.19/
    manifest.json                        <- {Ref, Digest, Layers:[{Digest, Path},...]}
    layers/
      sha256:abc.../                     <- extracted tar contents of layer 0 (base OS)
        bin/
        etc/
        usr/
        .done                            <- marker; presence = extraction complete
      sha256:def.../                     <- extracted tar contents of layer 1
        usr/local/bin/app
        .done
```

---

## OverlayFS Assembly: Mount Flow

```
Read /var/lib/thrive/images/{ref}/manifest.json -> []Layer

lowerParts = reversed layer paths
  -- top layer (highest index) first; OverlayFS takes precedence top-to-bottom
  -- e.g. ["layers/sha256:def/", "layers/sha256:abc/"]

lowerDirs = strings.Join(lowerParts, ":")

upperDir  = /var/lib/thrive/containers/{id}/upper/    <- container write layer
workDir   = /var/lib/thrive/containers/{id}/work/     <- kernel scratch (must be empty)
mergedDir = /var/lib/thrive/containers/{id}/merged/   <- unified rootfs view

os.MkdirAll(upperDir, workDir, mergedDir)

overlayOpts = "lowerdir=...,upperdir=...,workdir=..."

attempt:
  syscall.Mount("overlay", mergedDir, "overlay", 0, overlayOpts)
  -> success: return mergedDir

on EPERM or ENODEV (kernel overlay not available rootless):
  exec.Command("fuse-overlayfs", "-o", overlayOpts, mergedDir).Run()
  -> success: return mergedDir
  -> failure: return combined error (kernel err + fuse err)
```

**Why reverse layer order?**
OverlayFS `lowerdir` is specified with the highest-priority layer leftmost (leftmost wins).
OCI layers are ordered bottom-to-top (index 0 = base, last index = most recent). Reversing
makes the most recent layer highest priority, matching Docker semantics.

---

## fuse-overlayfs Fallback Path

When `syscall.Mount` returns `EPERM` (non-root, kernel < 5.11) or `ENODEV` (overlay module
not loaded), THRIVE falls back to the `fuse-overlayfs` binary:

```
exec.LookPath("fuse-overlayfs")  <- verify binary exists before attempting
exec.Command("fuse-overlayfs", "-o", overlayOpts, mergedDir)
```

`Unmount` calls `syscall.Unmount(mergedDir, syscall.MNT_DETACH)`, which notifies the FUSE
process to exit. `MNT_DETACH` (lazy unmount) prevents `EBUSY` when file handles are still
open at container exit time.

If `fuse-overlayfs` is not in PATH, `Mount` returns a combined error with a hint to install
the binary. See RISKS.md RISK-001.

---

## Push Flow

```
Read /var/lib/thrive/images/{ref}/manifest.json -> []Layer

for each layer:
  pr, pw = io.Pipe()
  go func:
    tw = tar.NewWriter(pw)
    filepath.Walk(layer.Path) -> add each file to tar stream
    tw.Close(); pw.Close()
  v1layer = tarball.LayerFromReader(pr)

img = mutate.AppendLayers(empty.Image, v1layers...)

remote.Write(parsed, img, remote.WithAuth(auth))
  <- uploads each layer blob, then pushes manifest JSON
```

Re-tarring from extracted directories means push works even when the original compressed
layer tarballs were discarded after pull. Re-tarred layers have different digests from the
originals (different compression) but identical content.

---

## Chunk Store Layout

Content-addressed storage for arbitrary byte blobs. Used by lazypull (on-demand FUSE fetch)
and P2P (chunk distribution across peers).

```
/var/lib/thrive/chunks/
  {xx}/          <- first 2 hex chars of SHA-256 digest (directory sharding)
    {rest}       <- remaining 62 hex chars of digest

Example:
  digest = "a3b4c5d6e7f8..." ->
  /var/lib/thrive/chunks/a3/b4c5d6e7f8...
```

Internal API:
- `Put(ctx, digest, data []byte) error` — write bytes; creates parent dir if needed
- `Get(ctx, digest) ([]byte, error)` — read bytes; returns `os.ErrNotExist` if missing
- `Has(ctx, digest) bool` — stat check only, no read

Directory sharding (first 2 hex chars = 256 possible subdirectories) keeps directory entry
counts bounded at scale, avoiding ext4/xfs performance degradation on large flat directories.
