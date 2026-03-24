# Migrate replicated-sdk from Wolfi to cve0.io — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all Wolfi/Chainguard package sources with cve0.io and eliminate the Dockerfile build path so all image builds use the same melange/apko pipeline.

**Architecture:** Swap repository URLs and keyring references in `deploy/apko.yaml` and `deploy/melange.yaml` from `packages.wolfi.dev` to `apk.cve0.io`. Rename `dagger/chainguard.go` to `dagger/image.go` with updated function names. Replace the Dockerfile-based test build with the melange/apko pipeline. Delete the Dockerfile.

**Tech Stack:** Go (Dagger pipeline), melange, apko, YAML configs

**Spec:** `docs/superpowers/specs/2026-03-24-migrate-wolfi-to-cve0-design.md`

---

### Task 1: Update deploy/apko.yaml to use cve0.io

**Files:**
- Modify: `deploy/apko.yaml`

- [ ] **Step 1: Update repository and keyring URLs, remove wolfi-baselayout**

In `deploy/apko.yaml`, make these changes:
- Line 3: `https://packages.wolfi.dev/os` → `https://apk.cve0.io`
- Line 6: `https://packages.wolfi.dev/os/wolfi-signing.rsa.pub` → `https://apk.cve0.io/key/cve0-signing.rsa.pub`
- Line 13: Remove `- wolfi-baselayout`

The file should look like:

```yaml
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

accounts:
  groups:
    - groupname: replicated
      gid: 1001
  users:
    - username: replicated
      uid: 1001
      gid: 1001
  run-as: replicated

environment:
  VERSION: 1.0.0

entrypoint:
  command: /replicated api
```

- [ ] **Step 2: Commit**

```bash
git add deploy/apko.yaml
git commit -m "chore: switch apko.yaml from wolfi to cve0.io packages"
```

---

### Task 2: Update deploy/melange.yaml to use cve0.io

**Files:**
- Modify: `deploy/melange.yaml`

- [ ] **Step 1: Update repository and keyring URLs**

In `deploy/melange.yaml`, make these changes:
- Line 12: `https://packages.wolfi.dev/os` → `https://apk.cve0.io`
- Line 14: `https://packages.wolfi.dev/os/wolfi-signing.rsa.pub` → `https://apk.cve0.io/key/cve0-signing.rsa.pub`

The `environment.contents` section should look like:

```yaml
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
  environment:
    GOMODCACHE: '/var/cache/melange'
    CGO_ENABLED: '0'
```

Everything else in the file stays the same.

- [ ] **Step 2: Commit**

```bash
git add deploy/melange.yaml
git commit -m "chore: switch melange.yaml from wolfi to cve0.io packages"
```

---

### Task 3: Rename dagger/chainguard.go to dagger/image.go and rename functions

**Files:**
- Rename: `dagger/chainguard.go` → `dagger/image.go`
- Modify: `dagger/image.go` (function renames)
- Modify: `dagger/publish.go` (update call sites)
- Modify: `dagger/test.go` (update call sites)

- [ ] **Step 1: Rename the file**

```bash
cd dagger
git mv chainguard.go image.go
```

- [ ] **Step 2: Rename functions in dagger/image.go**

In `dagger/image.go`, rename the two public functions:
- `buildAndPublishChainguardImage` → `buildImage` (line 12)
- `publishChainguardImage` → `publishImage` (line 52)

The function signatures become:

```go
func buildImage(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	version string,
) (*dagger.Directory, *dagger.Directory, *dagger.File, error) {
```

```go
func publishImage(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	amdPackages *dagger.Directory,
	armPackages *dagger.Directory,
	melangeKey *dagger.File,
	version string,
	imagePath string,
	username string,
	password *dagger.Secret,
	cosignKey *dagger.Secret,
	cosignPassword *dagger.Secret,
) (string, error) {
```

Also add a nil guard for `cosignKey` in `publishImage` to prevent a nil-pointer panic when called without cosign credentials (e.g., TTL builds). Find the SBOM attestation block (the `if !strings.Contains(manifest, "application/spdx+json")` check) and wrap the cosign attestation logic with a nil check:

```go
	if cosignKey != nil && !strings.Contains(manifest, "application/spdx+json") && !strings.Contains(manifest, "application/vnd.cyclonedx+json") {
```

This replaces the existing condition on what is currently line 213 of `chainguard.go`. The rest of the block stays the same.

`sanitizeVersionForMelange` stays as-is.

- [ ] **Step 3: Update call sites in dagger/publish.go**

In `dagger/publish.go`, update these lines:
- Line 48: `buildAndPublishChainguardImage(ctx, dag, source, version)` → `buildImage(ctx, dag, source, version)`
- Line 61: `publishChainguardImage(ctx, dag, source, ...)` → `publishImage(ctx, dag, source, ...)`
- Line 80: `publishChainguardImage(ctx, dag, source, ...)` → `publishImage(ctx, dag, source, ...)`
- Line 85: `publishChainguardImage(ctx, dag, source, ...)` → `publishImage(ctx, dag, source, ...)`
- Line 104: `publishChainguardImage(ctx, dag, source, ...)` → `publishImage(ctx, dag, source, ...)`
- Line 109: `publishChainguardImage(ctx, dag, source, ...)` → `publishImage(ctx, dag, source, ...)`

- [ ] **Step 4: Update call sites in dagger/test.go**

In `dagger/test.go`, update these lines:
- Line 102: `buildAndPublishChainguardImage(ctx, dag, source, version)` → `buildImage(ctx, dag, source, version)`
- Line 111: `publishChainguardImage(` → `publishImage(`

- [ ] **Step 5: Verify it compiles**

```bash
cd dagger && go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add dagger/image.go dagger/publish.go dagger/test.go
git commit -m "refactor: rename chainguard.go to image.go and update function names"
```

---

### Task 4: Replace Dockerfile-based build with melange/apko pipeline in dagger/build.go

**Files:**
- Modify: `dagger/build.go`
- Delete: `deploy/Dockerfile`

- [ ] **Step 1: Rewrite buildAndPushImageToTTL in dagger/build.go**

Replace the existing `buildAndPushImageToTTL` function (lines 11-29) with:

```go
func buildAndPushImageToTTL(
	ctx context.Context,
	source *dagger.Directory,
) (string, string, string, error) {
	now := time.Now().Format("20060102150405")
	version := now // used as melange version string only

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

Keep all existing imports (`context`, `dagger/...`, `fmt`, `strings`, `time`). The `strings` package is still used by `buildAndPushChartToTTL` in the same file.

- [ ] **Step 2: Delete deploy/Dockerfile**

```bash
git rm deploy/Dockerfile
```

- [ ] **Step 3: Verify it compiles**

```bash
cd dagger && go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add dagger/build.go deploy/Dockerfile
git commit -m "feat: replace Dockerfile build with melange/apko pipeline for TTL builds

All image builds (dev, test, staging, production) now use the same
melange/apko pipeline with cve0.io packages, ensuring test images
match production images exactly."
```

---

### Task 5: Verify and clean up

**Files:**
- Check: all modified files

- [ ] **Step 1: Verify full dagger module compiles**

```bash
cd dagger && go build ./...
```

Expected: no errors

- [ ] **Step 2: Verify no remaining wolfi references**

```bash
grep -r "wolfi\|chainguard\|cgr.dev" deploy/ dagger/
```

Expected: no matches (there should be zero references to wolfi, chainguard, or cgr.dev in deploy/ and dagger/)

- [ ] **Step 3: Verify no remaining old function name references**

```bash
grep -r "publishChainguardImage\|buildAndPublishChainguardImage" dagger/
```

Expected: no matches

- [ ] **Step 4: Review git diff to confirm all changes are correct**

```bash
git diff main --stat
```

Expected files:
- `deploy/Dockerfile` — deleted
- `deploy/apko.yaml` — modified
- `deploy/melange.yaml` — modified
- `dagger/chainguard.go` → `dagger/image.go` — renamed + modified
- `dagger/build.go` — modified
- `dagger/publish.go` — modified
- `dagger/test.go` — modified
