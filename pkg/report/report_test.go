package report

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_EncodeDecodeReport(t *testing.T) {
	req := require.New(t)

	var input Report

	// instance report
	input = &InstanceReport{
		Events: []InstanceReportEvent{
			createTestInstanceEvent(1234567890),
		},
	}

	encoded, err := EncodeReport(input)
	req.NoError(err)

	decoded, err := DecodeReport(encoded, input.GetType())
	req.NoError(err)

	req.Equal(input, decoded)

	// custom app metrics report
	input = &CustomAppMetricsReport{
		Events: []CustomAppMetricsReportEvent{
			createTestCustomAppMetricsEvent(1234567890),
		},
	}

	encoded, err = EncodeReport(input)
	req.NoError(err)

	decoded, err = DecodeReport(encoded, input.GetType())
	req.NoError(err)

	// since values are an interface, compare the json representation
	marshalledInput, err := json.MarshalIndent(input, "", "  ")
	req.NoError(err)

	marshalledDecoded, err := json.MarshalIndent(decoded, "", "  ")
	req.NoError(err)

	req.Equal(string(marshalledInput), string(marshalledDecoded))
}

func Test_AppendReport(t *testing.T) {
	req := require.New(t)

	instanceReportWithMaxEvents := getTestInstanceReportWithMaxEvents()
	instanceReportWithMaxSize, err := getTestInstanceReportWithMaxSize()
	req.NoError(err)

	customAppMetricsReportWithMaxEvents := getTestCustomAppMetricsReportWithMaxEvents()
	customAppMetricsReportWithMaxSize, err := getTestCustomAppMetricsReportWithMaxSize()
	req.NoError(err)

	tests := []struct {
		name           string
		existingReport Report
		newReport      Report
		wantReport     Report
	}{
		{
			name:           "instance report - no existing report",
			existingReport: nil,
			newReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
				},
			},
			wantReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
				},
			},
		},
		{
			name: "instance report - report exists with no events",
			existingReport: &InstanceReport{
				Events: []InstanceReportEvent{},
			},
			newReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
				},
			},
			wantReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
				},
			},
		},
		{
			name: "instance report - report exists with a few events",
			existingReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
				},
			},
			newReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(4),
					createTestInstanceEvent(5),
					createTestInstanceEvent(6),
				},
			},
			wantReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(1),
					createTestInstanceEvent(2),
					createTestInstanceEvent(3),
					createTestInstanceEvent(4),
					createTestInstanceEvent(5),
					createTestInstanceEvent(6),
				},
			},
		},
		{
			name:           "instance report - report exists with max number of events",
			existingReport: instanceReportWithMaxEvents,
			newReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit())),
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit() + 1)),
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit() + 2)),
				},
			},
			wantReport: &InstanceReport{
				Events: append(instanceReportWithMaxEvents.Events[3:], []InstanceReportEvent{
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit())),
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit() + 1)),
					createTestInstanceEvent(int64(instanceReportWithMaxEvents.GetEventLimit() + 2)),
				}...),
			},
		},
		{
			name:           "instance report - report exists with max report size",
			existingReport: instanceReportWithMaxSize,
			newReport: &InstanceReport{
				Events: []InstanceReportEvent{
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events))),
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events) + 1)),
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events) + 2)),
				},
			},
			wantReport: &InstanceReport{
				Events: append(instanceReportWithMaxSize.Events[3:], []InstanceReportEvent{
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events))),
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events) + 1)),
					createLargeTestInstanceEvent(int64(len(instanceReportWithMaxSize.Events) + 2)),
				}...),
			},
		},
		{
			name:           "custom app metrics report - no existing report",
			existingReport: nil,
			newReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
				},
			},
			wantReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
				},
			},
		},
		{
			name: "custom app metrics report - report exists with no events",
			existingReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{},
			},
			newReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
				},
			},
			wantReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
				},
			},
		},
		{
			name: "custom app metrics report - report exists with a few events",
			existingReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
				},
			},
			newReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(4),
					createTestCustomAppMetricsEvent(5),
					createTestCustomAppMetricsEvent(6),
				},
			},
			wantReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(1),
					createTestCustomAppMetricsEvent(2),
					createTestCustomAppMetricsEvent(3),
					createTestCustomAppMetricsEvent(4),
					createTestCustomAppMetricsEvent(5),
					createTestCustomAppMetricsEvent(6),
				},
			},
		},
		{
			name:           "custom app metrics report - report exists with max number of events",
			existingReport: customAppMetricsReportWithMaxEvents,
			newReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit())),
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit() + 1)),
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit() + 2)),
				},
			},
			wantReport: &CustomAppMetricsReport{
				Events: append(customAppMetricsReportWithMaxEvents.Events[3:], []CustomAppMetricsReportEvent{
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit())),
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit() + 1)),
					createTestCustomAppMetricsEvent(int64(customAppMetricsReportWithMaxEvents.GetEventLimit() + 2)),
				}...),
			},
		},
		{
			name:           "custom app metrics report - report exists with max report size",
			existingReport: customAppMetricsReportWithMaxSize,
			newReport: &CustomAppMetricsReport{
				Events: []CustomAppMetricsReportEvent{
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events))),
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events) + 1)),
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events) + 2)),
				},
			},
			wantReport: &CustomAppMetricsReport{
				Events: append(customAppMetricsReportWithMaxSize.Events[3:], []CustomAppMetricsReportEvent{
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events))),
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events) + 1)),
					createLargeTestCustomAppMetricsEvent(int64(len(customAppMetricsReportWithMaxSize.Events) + 2)),
				}...),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientsetObjects := []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      util.GetReplicatedDeploymentName(),
						Namespace: "default",
						UID:       "test-deployment-uid",
					},
				},
			}

			if tt.existingReport != nil {
				encoded, err := EncodeReport(tt.existingReport)
				req.NoError(err)

				clientsetObjects = append(clientsetObjects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.existingReport.GetSecretName(),
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       util.GetReplicatedDeploymentName(),
								UID:        "test-deployment-uid",
							},
						},
					},
					Data: map[string][]byte{
						tt.existingReport.GetSecretKey(): encoded,
					},
				})
			}

			clientset := fake.NewSimpleClientset(clientsetObjects...)

			err := AppendReport(clientset, "default", tt.newReport)
			req.NoError(err)

			// validate secret exists and has the expected data
			secret, err := clientset.CoreV1().Secrets("default").Get(context.TODO(), tt.wantReport.GetSecretName(), metav1.GetOptions{})
			req.NoError(err)
			req.NotNil(secret.Data[tt.wantReport.GetSecretKey()])
			req.Equal(string(secret.OwnerReferences[0].UID), "test-deployment-uid")

			gotReport, err := DecodeReport(secret.Data[tt.wantReport.GetSecretKey()], tt.wantReport.GetType())
			req.NoError(err)

			if tt.wantReport.GetType() == ReportTypeInstance {
				wantNumOfEvents := len(tt.wantReport.(*InstanceReport).Events)
				gotNumOfEvents := len(gotReport.(*InstanceReport).Events)

				if wantNumOfEvents != gotNumOfEvents {
					t.Errorf("want %d events, got %d", wantNumOfEvents, gotNumOfEvents)
					return
				}

				req.Equal(tt.wantReport, gotReport)
			} else {
				wantNumOfEvents := len(tt.wantReport.(*CustomAppMetricsReport).Events)
				gotNumOfEvents := len(gotReport.(*CustomAppMetricsReport).Events)

				if wantNumOfEvents != gotNumOfEvents {
					t.Errorf("want %d events, got %d", wantNumOfEvents, gotNumOfEvents)
					return
				}

				// since values of custom app metrics are an interface, compare the json representation
				wantJSON, err := json.MarshalIndent(tt.wantReport, "", "  ")
				req.NoError(err)

				gotJSON, err := json.MarshalIndent(gotReport, "", "  ")
				req.NoError(err)

				req.Equal(string(wantJSON), string(gotJSON))
			}
		})
	}
}

func createTestInstanceEvent(reportedAt int64) InstanceReportEvent {
	return InstanceReportEvent{
		ReportedAt:                reportedAt,
		LicenseID:                 "test-license-id",
		InstanceID:                "test-instance-id",
		ClusterID:                 "test-cluster-id",
		UserAgent:                 "test-user-agent",
		AppStatus:                 "ready",
		ResourceStates:            "[]",
		K8sVersion:                "1.29.0",
		K8sDistribution:           "test-distribution",
		DownstreamChannelID:       "test-channel-id",
		DownstreamChannelName:     "test-channel-name",
		DownstreamChannelSequence: 1,
		Tags:                      `{"force": false, "tags": {}}`,
	}
}

func createLargeTestInstanceEvent(seed int64) InstanceReportEvent {
	r := rand.New(rand.NewSource(seed))

	sizeInBytes := 100 * 1024 // 100KB

	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

	randomBytes := make([]byte, sizeInBytes)
	for i := 0; i < sizeInBytes; i++ {
		randomBytes[i] = charset[r.Intn(len(charset))]
	}

	return InstanceReportEvent{
		ResourceStates: string(randomBytes), // can use any field here
	}
}

func getTestInstanceReportWithMaxEvents() *InstanceReport {
	report := &InstanceReport{
		Events: []InstanceReportEvent{},
	}
	for i := 0; i < report.GetEventLimit(); i++ {
		report.Events = append(report.Events, createTestInstanceEvent(int64(i)))
	}
	return report
}

func getTestInstanceReportWithMaxSize() (*InstanceReport, error) {
	report := &InstanceReport{
		Events: []InstanceReportEvent{},
	}

	encoded, err := EncodeReport(report)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode instance report")
	}

	for i := 0; len(encoded) <= report.GetSizeLimit(); i++ {
		seed := int64(i)
		event := createLargeTestInstanceEvent(seed)
		eventSize := len(event.ResourceStates)

		if len(encoded)+eventSize > report.GetSizeLimit() {
			break
		}

		report.Events = append(report.Events, event)

		encoded, err = EncodeReport(report)
		if err != nil {
			return nil, errors.Wrap(err, "failed to encode instance report")
		}
	}

	return report, nil
}

func createTestCustomAppMetricsEvent(reportedAt int64) CustomAppMetricsReportEvent {
	return CustomAppMetricsReportEvent{
		ReportedAt: reportedAt,
		LicenseID:  "test-license-id",
		InstanceID: "test-instance-id",
		Data: map[string]interface{}{
			"key1_string":         "val1",
			"key2_int":            2,
			"key3_float":          3.5,
			"key4_numeric_string": "4.0",
			"key5_bool":           true,
		},
	}
}

func createLargeTestCustomAppMetricsEvent(seed int64) CustomAppMetricsReportEvent {
	r := rand.New(rand.NewSource(seed))

	sizeInBytes := 100 * 1024 // 100KB

	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

	randomBytes := make([]byte, sizeInBytes)
	for i := 0; i < sizeInBytes; i++ {
		randomBytes[i] = charset[r.Intn(len(charset))]
	}

	return CustomAppMetricsReportEvent{
		Data: map[string]interface{}{
			"random_bytes": randomBytes,
		},
	}
}

func getTestCustomAppMetricsReportWithMaxEvents() *CustomAppMetricsReport {
	report := &CustomAppMetricsReport{
		Events: []CustomAppMetricsReportEvent{},
	}
	for i := 0; i < report.GetEventLimit(); i++ {
		report.Events = append(report.Events, createTestCustomAppMetricsEvent(int64(i)))
	}
	return report
}

func getTestCustomAppMetricsReportWithMaxSize() (*CustomAppMetricsReport, error) {
	report := &CustomAppMetricsReport{
		Events: []CustomAppMetricsReportEvent{},
	}

	encoded, err := EncodeReport(report)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode custom app metrics report")
	}

	for i := 0; len(encoded) <= report.GetSizeLimit(); i++ {
		seed := int64(i)
		event := createLargeTestCustomAppMetricsEvent(seed)
		eventSize := len(event.Data["random_bytes"].([]byte))

		if len(encoded)+eventSize > report.GetSizeLimit() {
			break
		}

		report.Events = append(report.Events, event)

		encoded, err = EncodeReport(report)
		if err != nil {
			return nil, errors.Wrap(err, "failed to encode custom app metrics report")
		}
	}

	return report, nil
}
