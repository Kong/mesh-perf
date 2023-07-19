local g = import 'g.libsonnet';

local timeSeries = g.panel.timeSeries;

{
  milliseconds(title, desc, targets):
    timeSeries.new(title)
    + timeSeries.withDescription(desc)
    + timeSeries.queryOptions.withTargets(targets)
    + timeSeries.standardOptions.withUnit('ms'),
}
