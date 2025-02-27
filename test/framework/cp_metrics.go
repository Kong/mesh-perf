package framework

import (
	"context"
	"errors"
	"syscall"
	"time"
)

func XdsDeliveryCount(ctx context.Context, promClient *PromClient) (int, error) {
	return promClient.QueryIntValue(ctx, "xds_delivery_count")
}

func XdsAckRequestsReceived(ctx context.Context, promClient *PromClient) (int, error) {
	return promClient.QueryIntValue(ctx, `sum(xds_requests_received{confirmation="ACK"})`)
}

func WatchXdsDeliveryCount(
	ctx context.Context,
	promClient *PromClient,
	stopCh <-chan struct{},
	metricCh chan<- int,
	errCh chan<- error,
) {
	lastVal := -1 // unreachable value for counter

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		val, err := XdsDeliveryCount(ctx, promClient)
		switch {
		case errors.Is(err, syscall.ECONNREFUSED):
		case errors.Is(err, ErrNoResults):
		case err != nil:
			errCh <- err
			return
		case lastVal != val:
			metricCh <- val
			lastVal = val
		}

		time.Sleep(3 * time.Second)
	}
}
