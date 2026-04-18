---
title: Composition contract
---

A composition is a directory named after a component type. The directory defines both the validation contract and the execution contract for that type.

## Required files

Each composition directory is expected to contain:

- `schema.yaml`
- `job.yaml`

## Schema contract

The schema is a JSON Schema document that validates the component's type-specific inputs.

```yaml
$schema: http://json-schema.org/draft-07/schema#
title: Helm Component Type
type: object
properties:
  type:
    const: helm
  inputs:
    type: object
    properties:
      chart:
        type: string
      timeout:
        type: string
```

Use the schema to define required fields, enums, defaults, and bounds.

## Job registry contract

The job registry defines one or more executable jobs for the type.

```yaml
apiVersion: sourceplane.io/v1
kind: JobRegistry
metadata:
  name: helm-jobs
jobs:
  - name: deploy
    runsOn: ubuntu-22.04
    timeout: 15m
    retries: 2
    steps:
      - name: deploy
        run: helm upgrade --install {{.Component}} {{.chart}}
```

## Template inputs

Job steps resolve against merged component data. In the built-in compositions, common template values include:

- `.Component`
- merged input fields such as `.chart` or `.namespacePrefix`

Keep templates simple and deterministic. If the same input is required across many components, express it as schema or job defaults instead of repeating it in every component manifest.

## Execution semantics

Jobs can declare:

- `runsOn`
- `timeout`
- `retries`
- `labels`
- `steps`

Steps can declare:

- `run` or `use`
- `phase` and `order`
- `retry`
- `timeout`
- `onFailure`

The compiler resolves those fields into the plan artifact, and the runtime consumes them later.