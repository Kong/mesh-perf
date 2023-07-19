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
    panels.milliseconds('Pod Startup Time', 'Average time between Pod creation and readiness', queries.avgPodTimeToStart(ds.uid))
  ])
]))
