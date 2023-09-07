package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if len(os.Args) <= 1 {
		logger.Error("expected filenames as arguments")
		os.Exit(1)
	}

	cfg := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(cfg)
	logsApi := datadogV2.NewLogsApi(apiClient)
	ctx := datadog.NewDefaultContext(context.Background())

	var logs []datadogV2.HTTPLogItem
	for _, reportFilename := range os.Args[1:] {
		contents, err := os.ReadFile(reportFilename)
		if err != nil {
			logger.Error("couldn't read results file", "filename", reportFilename, "error", err)
			continue
		}
		logs = append(logs, datadogV2.HTTPLogItem{
			Hostname: datadog.PtrString("github-actions"),
			Message:  string(contents),
			Service:  datadog.PtrString("mesh-perf-test"),
		})
	}
	_, resp, err := logsApi.SubmitLog(ctx, logs)
	if resp.StatusCode != http.StatusAccepted {
		logger.Error(fmt.Sprintf("expected status code %d", http.StatusAccepted), "code", resp.StatusCode)
	}
	if err != nil {
		logger.Error("error submitting logs", "error", err)
		os.Exit(1)
	}
}
