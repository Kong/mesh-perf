package framework

import (
	"strings"
	"time"
)

func XdsDeliveryCount(promClient *PromClient) (int, error) {
	return promClient.QueryIntValue("xds_delivery_count")
}

func WatchXdsDeliveryCount(promClient *PromClient, stopCh <-chan struct{}, metricCh chan<- int, errCh chan<- error) {
	lastVal := -1 // unreachable value for counter
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		val, err := XdsDeliveryCount(promClient)
		if err != nil && !strings.Contains(err.Error(), "No results found for the query") {
			errCh <- err
			return
		}
		if lastVal != val {
			metricCh <- val
			lastVal = val
		}
		time.Sleep(3 * time.Second)
	}
}
