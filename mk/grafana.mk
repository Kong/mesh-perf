.PHONY: grafana-start
start-grafana:
	cd tools/observability && (docker-compose up --detach) && \
	echo "\nGrafana successfully started, access using URL:" && \
	$(TOP)/tools/observability/geturl.sh

destroy-grafana:
	cd tools/observability && docker-compose down
