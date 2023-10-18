package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_InstanceReportEncodeDecode(t *testing.T) {
	req := require.New(t)

	testReport := &InstanceReport{
		Events: []InstanceReportEvent{
			{
				ReportedAt:                1234567890,
				LicenseID:                 "test-license-id",
				InstanceID:                "test-instance-id",
				ClusterID:                 "test-cluster-id",
				AppStatus:                 "ready",
				ResourceStates:            "[]",
				K8sVersion:                "1.29.0",
				K8sDistribution:           "test-distribution",
				DownstreamChannelID:       "test-channel-id",
				DownstreamChannelName:     "test-channel-name",
				DownstreamChannelSequence: 1,
			},
		},
	}

	encoded, err := testReport.Encode()
	req.NoError(err)

	decoded, err := DecodeInstanceReport(encoded)
	req.NoError(err)

	req.Equal(testReport, decoded)
}
