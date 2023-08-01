package k8s_test

import (
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

type reportEntry struct {
	Time  int64  `yaml:"time"`
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type specReport struct {
	State         string `yaml:"state"`
	Description   string `yaml:"description"`
	ReportEntries []reportEntry
}

type report struct {
	Parameters       map[string]string `yaml:"parameters"`
	SuitePath        string            `yaml:"suitePath"`
	SuiteDescription string            `yaml:"suiteDescription"`
	SpecReports      []specReport      `yaml:"specReports"`
}

func makeReport(ginkgoReport ginkgo.Report) report {
	parameters := map[string]string{}
	for _, envKeyVal := range os.Environ() {
		if strings.HasPrefix(envKeyVal, "PERF_TEST") {
			assignment := strings.SplitN(envKeyVal, "=", 2)
			parameters[assignment[0]] = assignment[1]
		}
	}

	report := report{
		Parameters:       parameters,
		SuitePath:        ginkgoReport.SuitePath,
		SuiteDescription: ginkgoReport.SuiteDescription,
	}

	for _, rep := range ginkgoReport.SpecReports {
		if rep.LeafNodeType != types.NodeTypeIt {
			continue
		}
		specReport := specReport{
			State:       rep.State.String(),
			Description: rep.FullText(),
		}
		for _, entry := range rep.ReportEntries {
			specReport.ReportEntries = append(
				specReport.ReportEntries,
				reportEntry{
					Time:  entry.Time.Unix(),
					Name:  entry.Name,
					Value: entry.Value.String(),
				},
			)
		}
		report.SpecReports = append(
			report.SpecReports,
			specReport,
		)
	}

	return report
}