package report

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
)

var customAppMetricsReportMtx = sync.Mutex{}

type CustomAppMetricsReport struct {
	Events []CustomAppMetricsReportEvent `json:"events"`
}

type CustomAppMetricsReportEvent struct {
	ReportedAt int64                  `json:"reported_at"`
	LicenseID  string                 `json:"license_id"`
	InstanceID string                 `json:"instance_id"`
	Data       map[string]interface{} `json:"data"`
}

func (r *CustomAppMetricsReport) GetType() ReportType {
	return ReportTypeCustomAppMetrics
}

func (r *CustomAppMetricsReport) GetSecretName() string {
	return fmt.Sprintf(ReportSecretNameFormat, r.GetType())
}

func (r *CustomAppMetricsReport) GetSecretKey() string {
	return ReportSecretKey
}

func (r *CustomAppMetricsReport) AppendEvents(report Report) error {
	reportToAppend, ok := report.(*CustomAppMetricsReport)
	if !ok {
		return errors.Errorf("report is not a custom app metrics report")
	}

	r.Events = append(r.Events, reportToAppend.Events...)
	if len(r.Events) > r.GetEventLimit() {
		r.Events = r.Events[len(r.Events)-r.GetEventLimit():]
	}

	// remove one event at a time until the report is under the size limit
	encoded, err := EncodeReport(r)
	if err != nil {
		return errors.Wrap(err, "failed to encode report")
	}
	for len(encoded) > r.GetSizeLimit() {
		r.Events = r.Events[1:]
		if len(r.Events) == 0 {
			return errors.Errorf("size of latest event exceeds report size limit")
		}
		encoded, err = EncodeReport(r)
		if err != nil {
			return errors.Wrap(err, "failed to encode report")
		}
	}

	return nil
}

func (r *CustomAppMetricsReport) GetEventLimit() int {
	return ReportEventLimit
}

func (r *CustomAppMetricsReport) GetSizeLimit() int {
	return ReportSizeLimit
}

func (r *CustomAppMetricsReport) GetMtx() *sync.Mutex {
	return &customAppMetricsReportMtx
}
