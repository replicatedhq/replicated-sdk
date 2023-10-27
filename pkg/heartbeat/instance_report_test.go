package heartbeat

import (
	"context"
	"testing"

	heartbeattypes "github.com/replicatedhq/replicated-sdk/pkg/heartbeat/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_CreateInstanceReportEvent(t *testing.T) {
	testEvent := heartbeattypes.InstanceReportEvent{
		ReportedAt:                1234567890,
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
	}

	testReportWithOneEvent := &heartbeattypes.InstanceReport{
		Events: []heartbeattypes.InstanceReportEvent{testEvent},
	}
	testReportWithOneEventData, err := EncodeInstanceReport(testReportWithOneEvent)
	require.NoError(t, err)

	testReportWithMaxEvents := &heartbeattypes.InstanceReport{}
	for i := 0; i < InstanceReportEventLimit; i++ {
		testReportWithMaxEvents.Events = append(testReportWithMaxEvents.Events, testEvent)
	}
	testReportWithMaxEventsData, err := EncodeInstanceReport(testReportWithMaxEvents)
	require.NoError(t, err)

	type args struct {
		clientset kubernetes.Interface
		namespace string
		event     heartbeattypes.InstanceReportEvent
	}
	tests := []struct {
		name          string
		args          args
		wantNumEvents int
		wantErr       bool
	}{
		{
			name: "secret does not exist",
			args: args{
				clientset: fake.NewSimpleClientset(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      util.GetReplicatedDeploymentName(),
						Namespace: "default",
						UID:       "test-uid",
					},
				}),
				namespace: "default",
				event:     testEvent,
			},
			wantNumEvents: 1,
		},
		{
			name: "secret exists with an existing event",
			args: args{
				clientset: fake.NewSimpleClientset(
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedDeploymentName(),
							Namespace: "default",
							UID:       "test-uid",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      InstanceReportSecretName,
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       util.GetReplicatedDeploymentName(),
									UID:        "test-uid",
								},
							},
						},
						Data: map[string][]byte{
							InstanceReportSecretKey: testReportWithOneEventData,
						},
					},
				),
				namespace: "default",
				event:     testEvent,
			},
			wantNumEvents: 2,
		},
		{
			name: "secret exists without data",
			args: args{
				clientset: fake.NewSimpleClientset(
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedDeploymentName(),
							Namespace: "default",
							UID:       "test-uid",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      InstanceReportSecretName,
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       util.GetReplicatedDeploymentName(),
									UID:        "test-uid",
								},
							},
						},
					},
				),
				namespace: "default",
				event:     testEvent,
			},
			wantNumEvents: 1,
		},
		{
			name: "secret exists with max number of events",
			args: args{
				clientset: fake.NewSimpleClientset(
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.GetReplicatedDeploymentName(),
							Namespace: "default",
							UID:       "test-uid",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      InstanceReportSecretName,
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       util.GetReplicatedDeploymentName(),
									UID:        "test-uid",
								},
							},
						},
						Data: map[string][]byte{
							InstanceReportSecretKey: testReportWithMaxEventsData,
						},
					},
				),
				namespace: "default",
				event:     testEvent,
			},
			wantNumEvents: InstanceReportEventLimit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			err := CreateInstanceReportEvent(tt.args.clientset, tt.args.namespace, tt.args.event)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)

			// validate secret exists and has the expected data
			secret, err := tt.args.clientset.CoreV1().Secrets(tt.args.namespace).Get(context.TODO(), InstanceReportSecretName, metav1.GetOptions{})
			req.NoError(err)
			req.NotNil(secret.Data[InstanceReportSecretKey])

			report, err := DecodeInstanceReport(secret.Data[InstanceReportSecretKey])
			req.NoError(err)

			req.Len(report.Events, tt.wantNumEvents)

			for _, event := range report.Events {
				req.Equal(testEvent, event)
			}
		})
	}
}

func Test_InstanceReportEncodeDecode(t *testing.T) {
	req := require.New(t)

	testReport := &heartbeattypes.InstanceReport{
		Events: []heartbeattypes.InstanceReportEvent{
			{
				ReportedAt:                1234567890,
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
			},
		},
	}

	encoded, err := EncodeInstanceReport(testReport)
	req.NoError(err)

	decoded, err := DecodeInstanceReport(encoded)
	req.NoError(err)

	req.Equal(testReport, decoded)
}
