local g = import 'g.libsonnet';

local pq = g.query.prometheus;

{
  podTimeToStart(ds, quantile):
    pq.new(ds,
      'quantile(%s, kube_pod_status_ready_time - kube_pod_created{namespace="kuma-test"})' % quantile
    ) + pq.withLegendFormat(quantile),
  
  xdsDelivery(ds):
    pq.new(ds,
      'xds_delivery'
    ) + pq.withLegendFormat('{{quantile}}'),
    
  xdsGeneration(ds):
    pq.new(ds,
      'xds_generation{result="changed"}'
    ) + pq.withLegendFormat('{{quantile}}'),

  xdsGenerationRate(ds):
    pq.new(ds,
      'rate(xds_generation_count[$__rate_interval])'
    ) + pq.withLegendFormat('{{result}}'),
    
  kubeApiServerRequestLatency(ds, quantile):
    pq.new(ds,
      'histogram_quantile(%s, sum(rate(apiserver_request_duration_seconds_bucket{group="kuma.io"}[$__rate_interval])) by (le))' % quantile
    ) + pq.withLegendFormat(quantile),
    
  kumaStoreRequestLatency(ds, quantile):
    pq.new(ds,
      'histogram_quantile(%s, sum(rate(store_bucket[$__rate_interval])) by (le))' % quantile
    ) + pq.withLegendFormat(quantile),

  kumaStoreRequestRate(ds):
    pq.new(ds,
      'sum by (operation) (rate(store_count[$__rate_interval]))'
    ) + pq.withLegendFormat('{{operation}}'),

  kumaApiServerLatency(ds, quantile):
    pq.new(ds,
      'histogram_quantile(%s, sum(rate(api_server_http_request_duration_seconds_bucket[$__rate_interval])) by (le))' % quantile
    ) + pq.withLegendFormat(quantile),

  kumaClaCache(ds):
    pq.new(ds,
      'sum by (result) (rate(cla_cache[$__rate_interval]))'
    ) + pq.withLegendFormat('{{result}}'),

  kumaMeshCache(ds):
    pq.new(ds,
      'sum by (result) (rate(mesh_cache[$__rate_interval]))'
    ) + pq.withLegendFormat('{{result}}'),

  kubeAuthCache(ds):
    pq.new(ds,
      'sum by (result) (rate(kube_auth_cache[$__rate_interval]))'
    ) + pq.withLegendFormat('{{result}}'),

  controllerRuntimeReconcileLatency(ds, quantile):
    pq.new(ds,
      'histogram_quantile(%s, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller="pod"}[$__rate_interval])) by (le))' % quantile
    ) + pq.withLegendFormat(quantile),

  controllerRuntimeReconcileRate(ds):
    pq.new(ds,
      'sum by (result) (rate(controller_runtime_reconcile_total{controller="pod"}[$__rate_interval]))'
    ) + pq.withLegendFormat('{{result}}'),

}
