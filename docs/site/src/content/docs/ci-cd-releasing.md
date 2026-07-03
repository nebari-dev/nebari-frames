---
title: CI/CD and Releasing
---

## CI (`.github/workflows/ci.yml`)

Runs on every pull request and on push to `main`. Five jobs gate merges:

- **proto** - lints the protobuf definitions (`buf lint`) and fails if generated code (`gen/`) is stale relative to `proto/`.
- **go** - `golangci-lint` and `go test ./... -race -coverprofile=coverage.out`.
- **web** - installs the SPA's dependencies and runs `npm run lint`, `npm run typecheck`, and `npm test`.
- **chart** - `helm lint` plus two `helm template` renders (NebariApp mode and dev mode) to catch template errors before they reach a cluster.
- **e2e-sandbox** - builds the image, spins up a real k3d sandbox cluster (`nebari-dev/action-nebari-sandbox`) with the Nebari platform preinstalled, deploys Frames into it through an ArgoCD `Application` (dev-mode auth, since the sandbox has no trusted issuer for the pod to validate against), waits for the ArgoCD `Application` to go `Synced`/`Healthy`, and confirms the endpoint answers through the gateway. On failure it dumps the `NebariApp` conditions, pod state, and nebari-operator logs to help diagnose it - the same places described in [Troubleshooting](/troubleshooting/).

## Build Images (`.github/workflows/build-images.yaml`)

Runs on push to `main` and on `v*` tags. Builds one image and pushes it to **both** `ghcr.io/nebari-dev/nebari-frames` and `quay.io/nebari/nebari-frames`, tagged by branch, `sha-<sha>`, the literal git tag on tag pushes, and `latest` on the default branch. The image is smoke-tested (booted in dev mode, checked for an HTTP response) before either registry login happens, so a broken image never reaches a registry push, and a missing registry credential fails loudly next to the push step instead of silently skipping it.

## Release (`.github/workflows/release.yml`)

Triggered by pushing a `v*` tag. Three jobs:

- **test** - `go test ./... -race` as a release-time gate, independent of CI having already passed.
- **release-cli** - runs GoReleaser to build and publish the `frames` CLI for linux/darwin (amd64 and arm64) and windows/amd64 as GitHub release archives, and updates the `nebari-dev/homebrew-tap` formula.
- **release-chart** - copies `chart/` into the `nebari-dev/helm-repository` repo, stamps `Chart.yaml`'s `version` and `appVersion` from the tag, and commits and pushes - this is what publishes the chart consumed as `oci://quay.io/nebari/charts/nebari-frames` (see [Installation](/installation/)).

Release images are produced by `build-images.yaml`, which also triggers on `v*` tags - there is no duplicate image-building job in `release.yml`.

## Cutting a release

1. Confirm `main` is green on CI.
2. Push a tag matching `v*` (for example `v0.2.0`) to `main`.
3. `build-images.yaml` and `release.yml` both trigger off the tag: the former publishes the tagged image to both registries, the latter runs the release gate, publishes the CLI, and publishes the chart update.
4. Once all three land, both the image tag and the chart version are ready to reference from a values file or an ArgoCD `Application`.
