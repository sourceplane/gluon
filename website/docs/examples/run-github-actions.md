---
title: Run with GitHub Actions compatibility
---

The repository includes a minimal example that installs Helm through a GitHub Action and then uses the resulting binary from a later shell step.

## Compile the example plan

```bash
ciz plan \
  --intent examples/gha-actions/intent.yaml \
  --config-dir examples/gha-actions/compositions \
  --output /tmp/ciz-gha-actions-plan.json
```

## Execute the plan

```bash
ciz run \
  --plan /tmp/ciz-gha-actions-plan.json \
  --execute
```

Because the plan contains a `use:` step, `ciz run` auto-selects the `github-actions` backend unless you explicitly override it.

## Force the backend explicitly

```bash
ciz run \
  --plan /tmp/ciz-gha-actions-plan.json \
  --execute \
  --gha
```

Use the explicit flag when you want the command line itself to document that the plan requires GitHub Actions semantics.