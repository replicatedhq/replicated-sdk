// Package cache persists the license document and license-fields map to a
// dedicated Kubernetes Secret so the SDK can serve license endpoints when
// replicated.app is unreachable.
//
// The cache lives in a runtime-managed Secret (`replicated-license-cache`)
// rather than in the chart-managed `replicated` Secret, so helm upgrades
// don't clobber it. This follows the same pattern as the SDK's other
// out-of-band Secrets — `replicated-instance-report`,
// `replicated-custom-app-metrics-report`, `replicated-meta-data`, and
// `replicated-support-metadata` — none of which are rendered by any chart
// template either.
//
// Writes serialize through a package-level mutex (matching pkg/meta and
// pkg/supportbundle). Reads do not lock; concurrent reads of a Secret are
// safe. Read-only mode (store.GetReadOnlyMode()) suppresses writes so the
// SDK never attempts to create a Secret it doesn't have RBAC for.
package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pkg/errors"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretName is the name of the runtime-managed cache Secret. The chart's
// RBAC Role grants the SDK `update` on this exact resourceName.
const SecretName = "replicated-license-cache"

// Secret data keys.
const (
	KeyLicense       = "license"        // raw signed license YAML bytes
	KeyLicenseFields = "license-fields" // JSON-encoded LicenseFields map
	KeyLastFetched   = "last-fetched"   // RFC3339 timestamp of most recent successful upstream sync
)

// ErrLicenseCacheNotFound is returned by Read when the cache Secret does
// not exist or contains no usable license data. Callers use it to
// distinguish a cache miss from a transient k8s API failure.
var ErrLicenseCacheNotFound = errors.New("license cache not found")

// CachedLicense is the in-memory shape of a successful Read. LicenseFields
// is nil if the fields key was never written; LicenseBytes is always
// populated on a successful Read (the secret is considered "found" only
// when the license key is present).
type CachedLicense struct {
	LicenseBytes  []byte
	LicenseFields licensetypes.LicenseFields
	LastFetched   time.Time
}

// cacheLock serializes writes against the cache Secret to avoid lost
// updates when bootstrap and request-time syncs race. Reads are unlocked.
var cacheLock sync.Mutex

// Read returns the cached license, or ErrLicenseCacheNotFound if no usable
// cache exists. A Secret with no `license` key counts as not-found because
// the license document is the primary artifact — fields without a license
// to attach them to are not useful on their own.
func Read(ctx context.Context, clientset kubernetes.Interface, namespace string) (*CachedLicense, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, ErrLicenseCacheNotFound
		}
		return nil, errors.Wrap(err, "failed to get license cache secret")
	}

	licenseBytes, ok := secret.Data[KeyLicense]
	if !ok || len(licenseBytes) == 0 {
		return nil, ErrLicenseCacheNotFound
	}

	cached := &CachedLicense{
		LicenseBytes: licenseBytes,
	}

	if fieldsBytes, ok := secret.Data[KeyLicenseFields]; ok && len(fieldsBytes) > 0 {
		var fields licensetypes.LicenseFields
		if err := json.Unmarshal(fieldsBytes, &fields); err != nil {
			// Malformed fields blob is non-fatal — return the license
			// without fields and log so an operator can investigate.
			logger.Infof("license cache: failed to unmarshal cached license-fields, ignoring: %v", err)
		} else {
			cached.LicenseFields = fields
		}
	}

	if tsBytes, ok := secret.Data[KeyLastFetched]; ok && len(tsBytes) > 0 {
		if ts, err := time.Parse(time.RFC3339, string(tsBytes)); err == nil {
			cached.LastFetched = ts
		}
	}

	return cached, nil
}

// WriteLicense upserts the license bytes and refreshes the last-fetched
// timestamp. License-fields, if previously cached, are preserved.
//
// In read-only mode this is a no-op so the SDK never attempts a write it
// doesn't have RBAC for.
func WriteLicense(ctx context.Context, clientset kubernetes.Interface, namespace string, licenseBytes []byte) error {
	if len(licenseBytes) == 0 {
		return errors.New("refusing to cache empty license bytes")
	}
	return write(ctx, clientset, namespace, map[string][]byte{
		KeyLicense: licenseBytes,
	})
}

// WriteLicenseFields upserts the license-fields map and refreshes the
// last-fetched timestamp. The license document, if previously cached, is
// preserved.
func WriteLicenseFields(ctx context.Context, clientset kubernetes.Interface, namespace string, fields licensetypes.LicenseFields) error {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return errors.Wrap(err, "failed to marshal license fields")
	}
	return write(ctx, clientset, namespace, map[string][]byte{
		KeyLicenseFields: encoded,
	})
}

// write performs a partial-update upsert: existing keys not in `data` are
// preserved. A `last-fetched` timestamp is always refreshed alongside any
// successful update.
func write(ctx context.Context, clientset kubernetes.Interface, namespace string, data map[string][]byte) error {
	if store.GetStore().GetReadOnlyMode() {
		logger.Debugf("license cache: skipping write in read-only mode")
		return nil
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	merged := make(map[string][]byte, len(data)+1)
	for k, v := range data {
		merged[k] = v
	}
	merged[KeyLastFetched] = []byte(time.Now().UTC().Format(time.RFC3339))

	existing, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get license cache secret")
	}

	if kuberneteserrors.IsNotFound(err) {
		uid, err := util.GetReplicatedDeploymentUID(clientset, namespace)
		if err != nil {
			return errors.Wrap(err, "failed to get replicated deployment uid")
		}

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       util.GetReplicatedDeploymentName(),
						UID:        uid,
					},
				},
			},
			Data: merged,
		}

		if _, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return errors.Wrap(err, "failed to create license cache secret")
		}
		return nil
	}

	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}
	for k, v := range merged {
		existing.Data[k] = v
	}

	if _, err := clientset.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return errors.Wrap(err, "failed to update license cache secret")
	}
	return nil
}
