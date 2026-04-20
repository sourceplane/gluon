---
title: Installation
---

Install `gluon` from source when you want the local CLI, or run it as a packaged provider through `kiox` when you want workspace-pinned execution.

## Prerequisites

- macOS or Linux
- Go 1.25+ for source builds
- Docker only if you plan to use the Docker execution backend
- GitHub Actions only if you plan to rely on `use:` steps inside the GitHub Actions-compatible runner

## Build from this repository

Use this when you are working in the repository and want the local `./gluon` binary for examples and development.

```bash
make build
./gluon version
./gluon --help
```

## Install directly with Go

```bash
go install github.com/sourceplane/gluon/cmd/gluon@latest
```

Verify the CLI:

```bash
gluon version
gluon --help
```

## Install a release binary

Replace `<tag>` with the release tag you want to install.

```bash
# macOS arm64
curl -L https://github.com/sourceplane/gluon/releases/download/<tag>/gluon_<tag>_darwin_arm64.tar.gz | tar xz
sudo mv entrypoint /usr/local/bin/gluon
chmod +x /usr/local/bin/gluon

# Linux amd64
curl -L https://github.com/sourceplane/gluon/releases/download/<tag>/gluon_<tag>_linux_amd64.tar.gz | tar xz
sudo mv entrypoint /usr/local/bin/gluon
chmod +x /usr/local/bin/gluon
```

## Run gluon through kiox

This path is useful when you want the planner pinned as an OCI-distributed provider inside a reproducible workspace.

```bash
kiox init demo -p ghcr.io/sourceplane/gluon:<tag> as gluon
kiox --workspace demo -- gluon --help
```

## Build the docs site locally

The documentation site lives in `website/` and uses Docusaurus.

```bash
cd website
npm install
npm run docs:start
```

## Next steps

1. Follow the [quick start](./quick-start.md) to compile and preview the example plan.
2. Read [intent model](../concepts/intent-model.md) and [compositions](../concepts/compositions.md) before authoring your own contracts.