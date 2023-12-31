# Metrics

* Status: accepted

Technical Story: https://github.com/Kong/mesh-perf/issues/5

## Context and Problem Statement

We need to define set of metrics that we want to measure to have a consistent perf tests.

## Considered Options

* Set of metrics

## Decision Outcome

Chosen option: "Set of metrics"

## Pros and Cons of the Options

### Set of metrics

Resource utilization:
* CPU usage
* Memory usage

P50, P90, P99 of
* Time between applying Deployment and Pod be ready (exposed by Kubernetes?)
* XDS config delivery (time between setting the config into snapshot up to receiving ACK/NACK)
* XDS watchdog sync (time spent on generating XDS config)
* Kubernetes reconciles (Pod to Dataplane conversion, VIP generation)
* Latency of Kube API server responses (exposed by Kubernetes)
* Latency of DB operations (exposed by Kuma)
* Kuma API server responses
  * We can create a process in the background to generate requests (like DP request every 1s) to see what is the latency.
* Latency of tick for all Kuma components that have tickers (like insights, sub finalizer etc.)
  * It's not available yet.

Number of
* XDS reconciliations (consider counting real reconciliations i.e. only if mesh hash has changed)
* Kubernetes reconciliations
* Number of DB queries

Cache:
* Instrument all caches in the system with consistent metrics

The goal is to check perf of the CP, so we should not scrape DPP metrics for now.

### Scenarios

We should measure this set of metrics in the following scenarios
* A new deployment with nothing
* Deploy X services with Y replicas
* Trigger XDS changes by applying mesh wide policy (for example MeshRateLimit)
* Scale up a service to see how quickly a new endpoint is propagated (not to all services, because of reachable services)
* Enable mTLS to see how quickly we distribute certs

Each test case should end with sleep of 1 minute to stabilize environment and see if we are not generating additional changes.
