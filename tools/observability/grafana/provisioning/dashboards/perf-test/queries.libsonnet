local g = import 'g.libsonnet';

local pq = g.query.prometheus;

{
  avgPodTimeToStart(datasource):
    pq.new(
      datasource,
      'avg(kube_pod_status_ready_time - kube_pod_created{namespace="kuma-test"})'
    )
}
