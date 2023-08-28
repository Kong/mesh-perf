package framework

import (
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

type SpecReport struct {
	Parameters       map[string]string `json:"parameters"`
	SuitePath        string            `json:"suitePath"`
	SuiteDescription string            `json:"suiteDescription"`
	State            string            `json:"state"`
	Description      string            `json:"description"`
	ReportEntries    map[string]string `json:"reportEntries"`
}

func MakeSpecReports(ginkgoReport ginkgo.Report) []SpecReport {
	parameters := map[string]string{}
	for _, envKeyVal := range os.Environ() {
		if strings.HasPrefix(envKeyVal, "PERF_TEST") {
			assignment := strings.SplitN(envKeyVal, "=", 2)
			parameters[assignment[0]] = assignment[1]
		}
	}

	reports := []SpecReport{}
	for _, rep := range ginkgoReport.SpecReports {
		if rep.LeafNodeType != types.NodeTypeIt {
			continue
		}
		report := SpecReport{
			Parameters:       parameters,
			SuitePath:        ginkgoReport.SuitePath,
			SuiteDescription: ginkgoReport.SuiteDescription,
			State:            rep.State.String(),
			Description:      rep.FullText(),
			ReportEntries:    map[string]string{},
		}
		for _, entry := range rep.ReportEntries {
			report.ReportEntries[entry.Name] = entry.Value.String()
		}
		reports = append(reports, report)
	}

	return reports
}
