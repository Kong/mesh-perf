# Mesh Performance Tests

Performance tests of Kong Mesh.

## Run

1. Clone https://github.com/kumahq/kuma repo. Run from the Kuma directory
```sh
KIND_CLUSTER_NAME=kuma-1 make k3d/start
```

2. Run tests from mesh-perf directory
```sh
make run
```
