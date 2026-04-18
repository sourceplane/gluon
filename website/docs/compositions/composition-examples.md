---
title: Composition examples
---

The repository ships a small set of built-in compositions plus an example GitHub Actions composition used to validate the GitHub Actions-compatible runner.

## Built-in compositions

| Type | Purpose | Notable inputs |
| --- | --- | --- |
| `charts` | Package or publish Helm charts | `registry`, `pullPolicy` |
| `helm` | Deploy Helm-managed services | `chart`, `timeout`, `namespacePrefix`, `pullPolicy` |
| `helmCommon` | Deploy shared Helm-managed platform services | common service chart inputs and rollout settings |
| `terraform` | Apply Terraform-managed infrastructure | `workspace`, `autoApprove`, `parallelism` |

## Example-only composition

The repository also includes an example composition under `examples/gha-actions/compositions/gha-demo/`.

That example shows how a job can include a GitHub Actions `use:` step followed by ordinary shell commands:

```yaml
steps:
  - id: setup-demo
    name: setup-demo
    use: azure/setup-helm@v4.3.0
  - name: verify-gha-state
    run: |
      helm version --short
      which helm
```

Use that example as a reference when you need Actions-style setup behavior in a compiled plan.