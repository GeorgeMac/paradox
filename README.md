Paradox - InfluxDB Resource Controller
--------------------------------------

A Kubernetes controller for configuring and managing multiple InfluxDB (+ Cloud) Instances.
A control plane for managing multiple-instances of Influx; located inside and out of Kubernetes.

## Development

Currently, this is being developed locally against [kind](https://kind.sigs.k8s.io).

The current steps involved to create a cluster and install paradox are as follows:

1. Create a kind cluster `kind create cluster --name paradox`.
2. Ensure your current context is pointed at the new `kind-paradox` context.
3. Run `make install` to configure the CRDs against the cluster.
4. Run `make run` to start the controller.

See the [samples](./config/samples) directory for some example configuration.

These will need to be adjusted to point to and authenticate against either a cloud or local Influx instance.

## High-Level

- Create, manage and replicate[^1] Influx resources via a declarative API.
- Leverage common and established k8s tooling (e.g. kubectl) and standards.
- Tightly integrate Influx resources into Kubernetes.
  - Influx resource discovery via k8s APIs.
  - Automate Secret management (Influx token <-> k8s Secret).

[^1]: Replicate organizational and resource structure between instances (_not_ the time-series data it-self).
