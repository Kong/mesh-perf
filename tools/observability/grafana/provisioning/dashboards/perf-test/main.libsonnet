local g = import 'g.libsonnet';

local row = g.panel.row;

local panels = import './panels.libsonnet';
local queries = import './queries.libsonnet';

local ds = {type: 'prometheus', uid: 'Prometheus'};

g.dashboard.new('Perf Test')
+ g.dashboard.withDescription('Dashboards for Kong Mesh performance tests')
+ g.dashboard.withUid('perf-test')

+ g.dashboard.withAnnotations([
    g.dashboard.annotation.withDatasource(ds)
    + g.dashboard.annotation.withName('Test Execution')
    + g.dashboard.annotation.withIconColor('rgba(0, 211, 255, 1)')
    + g.dashboard.annotation.withEnable(true)
    + {expr: 'test_status', titleFormat: '{{spec}}'}
  ])

+ g.dashboard.withPanels(g.util.grid.makeGrid([

  row.new('Kubernetes')
  + row.withPanels([
    panels.milliseconds(
      'Pod Startup Time', 
      'Average time between Pod creation and readiness', 
      queries.avgPodTimeToStart(ds.uid)),
    panels.seconds(
      'Latency of Kube API server responses', 
      'The latency between a request sent from a client and a response returned by kube-apiserver', 
      [
        queries.kubeApiServerRequestLatency(ds.uid, "0.99"),
        queries.kubeApiServerRequestLatency(ds.uid, "0.90"),
        queries.kubeApiServerRequestLatency(ds.uid, "0.50")
      ]),
  ]),
  
  row.new('Kuma Control Plane')
  + row.withPanels([
    panels.milliseconds(
      'Latency of XDS config delivery', 
      'Time between setting the config into snapshot up to receiving ACK/NACK',
      queries.xdsDelivery(ds.uid)),
    panels.milliseconds(
      'Latency of XDS config generation', 
      'Time spent on generating XDS config',
      queries.xdsGeneration(ds.uid)),
    panels.seconds(
      'Latency of store operations',
      'The latency between a request sent from a Kuma CP and a response returned by store',
      [
        queries.kumaStoreRequestLatency(ds.uid, "0.99"),
        queries.kumaStoreRequestLatency(ds.uid, "0.90"),
        queries.kumaStoreRequestLatency(ds.uid, "0.50"),
      ]),
    panels.seconds(
      'Latency of Kuma API server responses',
      'The latency between a request sent from a client and a response returned by Kuma API Server',
      [
        queries.kumaApiServerLatency(ds.uid, "0.99"),
        queries.kumaApiServerLatency(ds.uid, "0.90"),
        queries.kumaApiServerLatency(ds.uid, "0.50"),
      ]),
    panels.opsPerSec(
      'Number of XDS reconciliations',
      'Number of snapshot reconciliations both when config changed and skipped',
      queries.xdsGenerationRate(ds.uid)),
    panels.opsPerSec(
      'Number of store operations',
      'Number of requests Kuma CP is producing against the store',
      queries.kumaStoreRequestRate(ds.uid)),
    panels.cacheHitMissRatio(
      'Endpoints cache performance',
      |||
      Cache protects ClusterLoadAssignments resources by sharing them between many goroutines which reconcile Dataplanes.

      hit - request was retrieved from the cache.

      hit-wait - request was retrieved from the cache after waiting for a concurrent request to fetch it from the database.

      miss - request was fetched from the database

      Refer to https://kuma.io/docs/latest/documentation/fine-tuning/#snapshot-generation
|||,
      queries.kumaClaCache(ds.uid)),
    panels.cacheHitMissRatio(
      'Mesh resources hash cache performance',
      |||
      Mesh Cache protects hashes calculated periodically for each Mesh in order to avoid the excessive generation of xDS resources.

      hit - request was retrieved from the cache.

      hit-wait - request was retrieved from the cache after waiting for a concurrent request to fetch it from the database.

      miss - request was fetched from the database

      Refer to https://kuma.io/docs/latest/documentation/fine-tuning/#snapshot-generation
|||,
      queries.kumaMeshCache(ds.uid)),
  ])
  
]))
