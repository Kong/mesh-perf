
# Why this package?

Code here is extracted from this module:
* github.com/gruntwork-io/terratest

We duplicated these functions from the module to remove the redundant and verbose logging. This logging is polluting the test output very much since we need to poll the cluster in certain mesh-perf test scenarios. 

Here is an example of the spam log entries

```
2024-12-12T10:11:43Z logger.go:67: Configuring Kubernetes client using config file /home/ubuntu/.kube/kind-kuma-1-config with context 
2024-12-12T10:11:43Z retry.go:91: Wait for pod postgres-release-postgresql-0 to be provisioned.
2024-12-12T10:11:43Z logger.go:67: Configuring Kubernetes client using config file /home/ubuntu/.kube/kind-kuma-1-config with context 
2024-12-12T10:11:43Z retry.go:103: Wait for pod postgres-release-postgresql-0 to be provisioned. returned an error: Pod postgres-release-postgresql-0 is not available, reason: , message: . Sleeping for 6s and will try again.
2024-12-12T10:11:49Z retry.go:91: Wait for pod postgres-release-postgresql-0 to be provisioned.
2024-12-12T10:11:49Z logger.go:67: Configuring Kubernetes client using config file /home/ubuntu/.kube/kind-kuma-1-config with context 
2024-12-12T10:11:49Z retry.go:103: Wait for pod postgres-release-postgresql-0 to be provisioned. returned an error: Pod postgres-release-postgresql-0 is not available, reason: , message: . Sleeping for 6s and will try again.
2024-12-12T10:11:55Z retry.go:91: Wait for pod postgres-release-postgresql-0 to be provisioned.
2024-12-12T10:11:55Z logger.go:67: Configuring Kubernetes client using config file /home/ubuntu/.kube/kind-kuma-1-config with context 
2024-12-12T10:11:55Z retry.go:103: Wait for pod postgres-release-postgresql-0 to be provisioned. returned an error: Pod postgres-release-postgresql-0 is not available, reason: , message: . Sleeping for 6s and will try again.
```

# Resolution

These duplications should be removed once terratest supports configuring logging verbosity for the Kubernetes client. 

We have a PR for the module to add support custom logger:
https://github.com/gruntwork-io/terratest/pull/1384
