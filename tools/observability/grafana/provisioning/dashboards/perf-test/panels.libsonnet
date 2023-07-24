local g = import 'g.libsonnet';

local timeSeries = g.panel.timeSeries;

{
  milliseconds(title, desc, targets):
    timeSeries.new(title)
    + timeSeries.withDescription(desc)
    + timeSeries.queryOptions.withTargets(targets)
    + timeSeries.standardOptions.withUnit('ms'),

  seconds(title, desc, targets):
    timeSeries.new(title)
    + timeSeries.withDescription(desc)
    + timeSeries.queryOptions.withTargets(targets)
    + timeSeries.standardOptions.withUnit('s'),

  opsPerSec(title, desc, targets):
    timeSeries.new(title)
    + timeSeries.withDescription(desc)
    + timeSeries.queryOptions.withTargets(targets)
    + timeSeries.standardOptions.withUnit('ops'),

  cacheHitMissRatio(title, desc, targets):
    timeSeries.new(title)
    + timeSeries.withDescription(desc)
    + timeSeries.queryOptions.withTargets(targets)
    + timeSeries.fieldConfig.defaults.custom.withStacking({mode: 'percent'})
    + timeSeries.fieldConfig.defaults.custom.withFillOpacity(25)
}
