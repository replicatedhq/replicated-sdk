# Migrate replicated-sdk from Wolfi to cve0.io

## Summary

Replace all Wolfi/Chainguard package sources with cve0.io (SecureBuild) across the build pipeline, and eliminate the separate Dockerfile build path so all image builds use the same melange/apko pipeline. This produces zero-CVE container images for all environments.

## Motivation

SecureBuild already builds and publishes a CVE-free `replicated-sdk` image at `cve0.io/replicated-sdk`. Rather than adding a separate integration with SecureBuild's API (which was attempted in PR #319 and reverted), we take the simpler approach: swap the package source in the existing melange/apko configs from Wolfi to cve0.io. The pipeline structure stays the same.

## Changes

### 1. Delete `deploy/Dockerfile`

The Dockerfile builds a separate image using `cgr.dev/chainguard/wolfi-base` and `cgr.dev/chainguard/static` for test/validation builds pushed to `ttl.sh`. This means tests run against a different image than production. Removing it and using the melange/apko pipeline for all builds ensures test and production images are identical.

### 2. Update `deploy/apko.yaml`

Switch package source from Wolfi to cve0:

```yaml
# Before
contents:
  repositories:
    - https://packages.wolfi.dev/os
    - ./packages/
  keyring:
    - https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
    - ./melange.rsa.pub
  packages:
    - replicated
    - bash
    - busybox
    - curl
    - wolfi-baselayout

# After
contents:
  repositories:
    - https://apk.cve0.io
    - ./packages/
  keyring:
    - https://apk.cve0.io/key/cve0-signing.rsa.pub
    - ./melange.rsa.pub
  packages:
    - replicated
    - bash
    - busybox
    - curl
```

Note: `wolfi-baselayout` is dropped. The cve0 replicated-sdk image spec does not use a baselayout package.

### 3. Update `deploy/melange.yaml`

Same repository/keyring swap for the build environment:

```yaml
# Before
environment:
  contents:
    repositories:
      - https://packages.wolfi.dev/os
    keyring:
      - https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
    packages:
      - ca-certificates-bundle
      - busybox
      - go
      - make

# After
environment:
  contents:
    repositories:
      - https://apk.cve0.io
    keyring:
      - https://apk.cve0.io/key/cve0-signing.rsa.pub
    packages:
      - ca-certificates-bundle
      - busybox
      - go
      - make
```

### 4. Rename `dagger/chainguard.go` to `dagger/image.go`

The file name references Chainguard but the functions use generic melange/apko tooling. Rename to `dagger/image.go` and rename functions:

| Current | New |
|---|---|
| `dagger/chainguard.go` | `dagger/image.go` |
| `buildAndPublishChainguardImage` | `buildImage` |
| `publishChainguardImage` | `publishImage` |

All call sites are updated:
- `dagger/publish.go` — `buildAndPublishChainguardImage` and `publishChainguardImage` calls
- `dagger/test.go` — `testSBOMGeneration` calls both `buildAndPublishChainguardImage` and `publishChainguardImage`
- `dagger/build.go` — new `buildAndPushImageToTTL` implementation

Helper function `sanitizeVersionForMelange` moves with the file rename (no changes needed).

### 5. Replace `buildAndPushImageToTTL` in `dagger/build.go`

Remove the Dockerfile-based build. Replace with a function that uses `buildImage` + `publishImage` targeting `ttl.sh`, matching what the dev publish path already does in `publish.go:61`.

The function signature stays the same — returns `(registry, repository, tag, error)` — so callers in `validate.go`, `testchart.go` don't change.

```go
func buildAndPushImageToTTL(
    ctx context.Context,
    source *dagger.Directory,
) (string, string, string, error) {
    now := time.Now().Format("20060102150405")
    version := now  // used as tag only, not a real version

    amdPackages, armPackages, melangeKey, err := buildImage(ctx, dag, source, version)
    if err != nil {
        return "", "", "", err
    }

    imagePath := fmt.Sprintf("ttl.sh/automated-%s/replicated-image/replicated-sdk", now)
    _, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey,
        version, imagePath, "", nil, nil, nil)
    if err != nil {
        return "", "", "", err
    }

    return "ttl.sh", fmt.Sprintf("automated-%s/replicated-image/replicated-sdk", now), "24h", nil
}
```

Note: The tag returned to callers is `24h` (matching the current behavior for ttl.sh image expiry). The `version` passed to `buildImage`/`publishImage` is the timestamp, used only for the melange version string. The repository path preserves the existing `replicated-image/replicated-sdk` structure.

### 6. No changes to chart, publish flow, or signing

| Component | Changes? |
|---|---|
| `chart/values.yaml` | No — keeps `proxy.replicated.com/library/replicated-sdk-image` |
| `dagger/publish.go` | Only function name updates (`publishImage` instead of `publishChainguardImage`) |
| `dagger/validate.go` | No — calls `buildAndPushImageToTTL` which is reimplemented internally |
| `dagger/testchart.go` | No — same interface |
| `dagger/build.go` (`buildAndPushChartToTTL`) | No — chart build function is unchanged |
| Cosign signing / SBOM | No — unchanged |
| SLSA provenance | No — unchanged |
| Registry targets | No — ttl.sh, Docker Hub, registry.replicated.com, registry.staging.replicated.com |

## Prerequisites (SecureBuild side)

1. **Go 1.26 package available in cve0 APK repo** — Resolved. `go-1.26.1` exists at `securebuildhq/securebuild-specs/packages/g/go/go/1.26.1`.
2. **Beta/alpha tag support** — The replicated-sdk package family version pattern `^(\d+)\.(\d+)(?:\.(\d+))?$` does not match pre-release tags like `1.7.0-beta.2`. This needs updating if beta releases resume. Last beta was June 2025, so this is low priority but should be tracked.

## Risks

- **Missing cve0 packages**: If a package needed by `apko.yaml` or `melange.yaml` is not in `apk.cve0.io`, the build fails at melange/apko time. This is visible and fixable.
- **Test build speed**: The Dockerfile path was single-arch (amd64 only) and faster. The melange/apko pipeline builds multi-arch (amd64 + arm64), roughly doubling build time. This is a tradeoff for testing the actual production image. If CI time becomes a concern, `publishImage` could be called with amd64-only for TTL builds.
- **SBOM overhead on test builds**: `publishImage` generates SBOMs and runs crane manifest inspection even for ephemeral TTL images. This is unnecessary overhead but keeps the code path simple. A lightweight publish path could be added later if needed.
- **Package version differences**: cve0 packages may have different versions than Wolfi (e.g., `busybox`, `curl`). The application should be compatible but may need testing.
- **Go package resolution**: Confirm that the `go` package in `apk.cve0.io` resolves to Go 1.26. The Dockerfile explicitly uses `go-1.26`; the melange.yaml uses `go` without version pinning.

## Rollback

If cve0.io has an outage or missing packages, revert the apko.yaml and melange.yaml changes back to Wolfi repos, and restore the Dockerfile. All changes are in `deploy/` and `dagger/` — no chart or application code changes are involved.

## Files changed

| File | Action |
|---|---|
| `deploy/Dockerfile` | Delete |
| `deploy/apko.yaml` | Update repos/keyring, drop `wolfi-baselayout` |
| `deploy/melange.yaml` | Update repos/keyring |
| `dagger/chainguard.go` | Rename to `dagger/image.go`, rename functions |
| `dagger/build.go` | Rewrite `buildAndPushImageToTTL` to use melange/apko |
| `dagger/publish.go` | Update function call names |
| `dagger/test.go` | Update function call names |
