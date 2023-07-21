package framework

func XdsDeliveryCount(promClient *PromClient) (int, error) {
	return promClient.QueryIntValue("xds_delivery_count")
}
