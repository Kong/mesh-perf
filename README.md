# Mesh Performance Tests

Performance tests of Kong Mesh.

## Run

1. Install dependencies
```sh
make dev/tools
```

1. Create local cluster
```sh
ENV=local make start-cluster
```

2. Run tests from mesh-perf directory
```sh
make run
```

3. Destroy local cluster
```sh
ENV=local make destroy-cluster
```
