---
title: Quickstart
---

Publish your first Frame and read it back, in about five minutes.

## Prerequisites

- The `frames` CLI installed and on your `PATH`.
- A running Nebari Frames registry to point the CLI at - a Nebari cluster with Frames installed (see [Installation](/installation/)), or your own `make dev` instance for a local try-out (see [Local Development](/local-development/)).

## Install the CLI

Install from the Nebari Homebrew tap:

```bash
brew install nebari-dev/tap/frames
```

Or download a prebuilt binary for your platform from the [releases page](https://github.com/nebari-dev/nebari-frames/releases) and put `frames` on your `PATH`.

Point the CLI at your registry (skip this if you are running the default `http://localhost:8080` from `make dev`):

```bash
frames config set api_url https://frames.example.com
```

## Log in

```bash
frames auth login
```

This starts an OIDC device-code flow: open the printed URL, approve the login, and the CLI caches your credentials under `~/.config/frames/`.

> Running against a `make dev` instance? Dev mode has no login step - `frames auth login` is not needed, you are already `dev-user`.

## Author a Frame

A Frame is a directory containing a `frame.yaml`. Create one:

```bash
mkdir my-frame && cd my-frame
cat > frame.yaml <<'EOF'
name: my-frame
description: A short Frame to try the registry with.
content: |
  # My Frame
  Say hello to Nebari Frames.
EOF
```

## Publish it

```bash
frames publish --dir . --changelog "Initial version"
```

On success the CLI prints the published name and version, for example `Published my-frame@1`.

## Browse and resolve

List the Frames you can read:

```bash
frames list
```

Print a Frame's metadata and content:

```bash
frames show <org>/my-frame
```

Print the inheritance-resolved form (parent content composed in) of a Frame:

```bash
frames resolve <org>/my-frame
```

## Next steps

- [Installation](/installation/) for deploying the registry itself on a Nebari cluster.
- [Architecture](/architecture/) for how publish, resolve, and the MCP endpoint fit together.
