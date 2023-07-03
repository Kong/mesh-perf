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

4. Destroy local cluster
```sh
ENV=local make destroy-cluster
```


## Setup EKS cluster from your machine

It is recommended to use saml2aws for AWS authorization. After authorizing you just need to run command

```sh
AWS_PROFILE=saml ENV=eks make start-cluster
```