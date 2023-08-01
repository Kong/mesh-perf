package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/go-logr/stdr"
)

func main() {
	logger := stdr.New(nil)
	if len(os.Args) <= 1 {
		logger.Error(nil, "expected filenames as arguments")
		os.Exit(1)
	}

	cfg := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(cfg)
	logsApi := datadogV2.NewLogsApi(apiClient)
	ctx := datadog.NewDefaultContext(nil)

	var logs []datadogV2.HTTPLogItem
	for _, reportFilename := range os.Args[1:] {
		contents, err := os.ReadFile(reportFilename)
		if err != nil {
			logger.Error(err, "couldn't read results file", "filename", reportFilename)
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
		logger.Error(nil, fmt.Sprintf("expected status code %d", http.StatusAccepted), "code", resp.StatusCode)
	}
	if err != nil {
		logger.Error(err, "error submitting logs")
		os.Exit(1)
	}
}
