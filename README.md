# Mesh Performance Tests

Performance tests of Kong Mesh.

## Run

1. Install dependencies
```sh
make dev/tools
```

2. Create local cluster
```sh
ENV=local make start-cluster
```

3. Run tests from mesh-perf directory
```sh
make run
```

You may want to:
```shell
 KMESH_LICENSE=$(pwd)/../kong-mesh/test/license/license.json PERF_TEST_MESH_VERSION=0.0.0-preview.ve242fefd0 ENV=local make run
```
Where kong-mesh is the checkout of kong-mesh

You can obtain the latest preview with:
```shell
curl -s -L https://docs.konghq.com/mesh/installer.sh  | VERSION=preview sh -s - --print-version | tail -n1
```

4. Destroy local cluster
```sh
ENV=local make destroy-cluster
```


## Setup EKS cluster from your machine

It is recommended to use saml2aws for AWS authorization. After authorizing you just need to run command

```sh
AWS_PROFILE=saml ENV=eks make start-cluster
```

## Observability

Observability tool is a way to inspect the end result of perf tests.
Perf test ends with snapshot of Prometheus TSDB save on the host which run the perf test (defaults to `/tmp/prom-snapshots`).
This directory will look like this
```
❯❯❯ ll -la /tmp/prom-snapshots/
total 0
drwxr-xr-x   6 jakub  wheel   192B Jun 29 15:40 ./
drwxrwxrwt  15 root   wheel   480B Jun 29 14:30 ../
drwxr-xr-x   6 jakub  wheel   192B Jun 29 15:28 20230629T125736Z-5c8c90f181c0b57f/
drwxr-xr-x   3 jakub  wheel    96B Jun 29 15:30 20230629T133034Z-77fee4f8e5a90c89/
drwxr-xr-x   3 jakub  wheel    96B Jun 29 15:33 20230629T133316Z-5e37819462543e4f/
drwxr-xr-x   3 jakub  wheel    96B Jun 29 15:40 20230629T134058Z-035f3439076d9f04/
```

You can run Docker Compose of Prometheus + Grafana with the data from test.

```sh
PROM_SNAPSHOT_PATH=/tmp/prom-snapshots/20230629T134058Z-035f3439076d9f04 make start-grafana
```

Grafana will be forwarded to `localhost:3000`. Kuma CP dashboard should be ready.

To update `kuma-cp.json` dashboard:
* place `mesh-perf` project next to `kuma`
* run `make upgrade/dashboards` from the top level directory of `mesh-perf`.
