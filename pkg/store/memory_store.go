package store

import (
	"strings"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

type InMemoryStore struct {
	replicatedID          string
	appID                 string
	license               *kotsv1beta1.License
	licenseFields         sdklicensetypes.LicenseFields
	appSlug               string
	appName               string
	channelID             string
	channelName           string
	channelSequence       int64
	releaseSequence       int64
	releaseCreatedAt      string
	releaseNotes          string
	versionLabel          string
	replicatedAppEndpoint string
	releaseImages         []string
	namespace             string
	appStatus             appstatetypes.AppStatus
	updates               []upstreamtypes.ChannelRelease
	// podImages holds namespace -> podUID -> []ImageInfo
	podImages map[string]map[string][]appstatetypes.ImageInfo
}

type InitInMemoryStoreOptions struct {
	ReplicatedID          string
	AppID                 string
	License               *kotsv1beta1.License
	LicenseFields         sdklicensetypes.LicenseFields
	AppName               string
	ChannelID             string
	ChannelName           string
	ChannelSequence       int64
	ReleaseSequence       int64
	ReleaseCreatedAt      string
	ReleaseNotes          string
	VersionLabel          string
	ReplicatedAppEndpoint string
	ReleaseImages         []string
	Namespace             string
}

func InitInMemory(options InitInMemoryStoreOptions) {
	SetStore(&InMemoryStore{
		replicatedID:          options.ReplicatedID,
		appID:                 options.AppID,
		appSlug:               options.License.Spec.AppSlug,
		license:               options.License,
		licenseFields:         options.LicenseFields,
		appName:               options.AppName,
		channelID:             options.ChannelID,
		channelName:           options.ChannelName,
		channelSequence:       options.ChannelSequence,
		releaseSequence:       options.ReleaseSequence,
		releaseCreatedAt:      options.ReleaseCreatedAt,
		releaseNotes:          options.ReleaseNotes,
		versionLabel:          options.VersionLabel,
		replicatedAppEndpoint: options.ReplicatedAppEndpoint,
		releaseImages:         options.ReleaseImages,
		namespace:             options.Namespace,
	})
}

func (s *InMemoryStore) GetReplicatedID() string {
	return s.replicatedID
}

func (s *InMemoryStore) GetAppID() string {
	return s.appID
}

func (s *InMemoryStore) GetLicense() *kotsv1beta1.License {
	return s.license
}

func (s *InMemoryStore) SetLicense(license *kotsv1beta1.License) {
	s.license = license.DeepCopy()
}

func (s *InMemoryStore) GetLicenseFields() sdklicensetypes.LicenseFields {
	return s.licenseFields
}

func (s *InMemoryStore) SetLicenseFields(licenseFields sdklicensetypes.LicenseFields) {
	// copy by value not reference
	if licenseFields == nil {
		s.licenseFields = nil
		return
	}
	if s.licenseFields == nil {
		s.licenseFields = sdklicensetypes.LicenseFields{}
	}
	for k, v := range licenseFields {
		s.licenseFields[k] = v
	}
}

func (s *InMemoryStore) IsDevLicense() bool {
	return s.license.Spec.LicenseType == "dev"
}

func (s *InMemoryStore) GetAppSlug() string {
	return s.appSlug
}

func (s *InMemoryStore) GetAppName() string {
	return s.appName
}

func (s *InMemoryStore) GetChannelID() string {
	return s.channelID
}

func (s *InMemoryStore) GetChannelName() string {
	return s.channelName
}

func (s *InMemoryStore) GetChannelSequence() int64 {
	return s.channelSequence
}

func (s *InMemoryStore) GetReleaseSequence() int64 {
	return s.releaseSequence
}

func (s *InMemoryStore) GetReleaseCreatedAt() string {
	return s.releaseCreatedAt
}

func (s *InMemoryStore) GetReleaseNotes() string {
	return s.releaseNotes
}

func (s *InMemoryStore) GetVersionLabel() string {
	return s.versionLabel
}

func (s *InMemoryStore) GetReplicatedAppEndpoint() string {
	return s.replicatedAppEndpoint
}

func (s *InMemoryStore) GetReleaseImages() []string {
	return s.releaseImages
}

func (s *InMemoryStore) GetNamespace() string {
	return s.namespace
}

func (s *InMemoryStore) GetAppStatus() appstatetypes.AppStatus {
	return s.appStatus
}

func (s *InMemoryStore) SetAppStatus(status appstatetypes.AppStatus) {
	s.appStatus = status
}

func (s *InMemoryStore) SetPodImages(namespace string, podUID string, images []appstatetypes.ImageInfo) {
	if s.podImages == nil {
		s.podImages = make(map[string]map[string][]appstatetypes.ImageInfo)
	}
	if s.podImages[namespace] == nil {
		s.podImages[namespace] = make(map[string][]appstatetypes.ImageInfo)
	}

	// If releaseImages are configured, filter by name:tag only and
	// normalize docker hub references so that, for example,
	// "nginx:alpine" matches "docker.io/library/nginx:alpine" and
	// "alpine/curl:latest" matches "docker.io/alpine/curl:latest".
	var filtered []appstatetypes.ImageInfo
	if len(s.releaseImages) > 0 {
		allowed := make(map[string]struct{}, len(s.releaseImages))
		for _, img := range s.releaseImages {
			if img == "" {
				continue
			}
			// Drop any @sha... suffix and normalize docker hub references
			canonical := canonicalNameTag(img)
			allowed[canonical] = struct{}{}
		}
		for _, info := range images {
			if info.Name == "" {
				continue
			}
			// Normalize the runtime reported image for comparison only.
			if _, ok := allowed[canonicalNameTag(info.Name)]; ok {
				filtered = append(filtered, info)
			}
		}
	} else {
		filtered = images
	}

	// Copy values to avoid external mutation
	copied := make([]appstatetypes.ImageInfo, len(filtered))
	copy(copied, filtered)
	s.podImages[namespace][podUID] = copied
}

// canonicalNameTag returns a canonical key for "name:tag" comparison.
// It removes any content digest suffix ("@sha256:...") and normalizes
// docker hub references by removing leading "docker.io/" or
// "index.docker.io/" and an optional leading "library/" namespace.
// If the reference does not include a tag, ":latest" is appended.
// Examples:
//
//	canonicalNameTag("nginx:latest") => "nginx:latest"
//	canonicalNameTag("docker.io/library/nginx:latest") => "nginx:latest"
//	canonicalNameTag("docker.io/nginx:latest") => "nginx:latest"
//	canonicalNameTag("docker.io/alpine/curl:latest") => "alpine/curl:latest"
func canonicalNameTag(s string) string {
	// strip content digest if present
	if at := strings.LastIndex(s, "@"); at != -1 {
		s = s[:at]
	}
	// normalize docker hub hostnames
	s = strings.TrimPrefix(s, "index.docker.io/")
	s = strings.TrimPrefix(s, "docker.io/")
	// if it is a docker hub library image, drop the implicit namespace
	// Only drop the library/ namespace for docker hub images.
	// At this point, we have already trimmed docker.io, so this is safe for hub
	// or for shorthand references that omit the registry host entirely.
	s = strings.TrimPrefix(s, "library/")
	// ensure a tag exists; if not, default to latest
	// a tag is present if there is a ':' after the last '/'
	lastSlash := strings.LastIndexByte(s, '/')
	lastColon := strings.LastIndexByte(s, ':')
	if lastColon == -1 || lastColon < lastSlash {
		s += ":latest"
	}
	return s
}

func (s *InMemoryStore) DeletePodImages(namespace string, podUID string) {
	if s.podImages == nil {
		return
	}
	if s.podImages[namespace] == nil {
		return
	}
	delete(s.podImages[namespace], podUID)
	if len(s.podImages[namespace]) == 0 {
		delete(s.podImages, namespace)
	}
}

func (s *InMemoryStore) GetRunningImages() map[string][]string {
	// Aggregate image -> unique SHA set across all namespaces/pods
	resultSet := make(map[string]map[string]struct{})
	for _, pods := range s.podImages {
		for _, images := range pods {
			for _, info := range images {
				if info.Name == "" || info.SHA == "" {
					continue
				}
				if _, ok := resultSet[info.Name]; !ok {
					resultSet[info.Name] = make(map[string]struct{})
				}
				resultSet[info.Name][info.SHA] = struct{}{}
			}
		}
	}
	out := make(map[string][]string, len(resultSet))
	for name, shas := range resultSet {
		list := make([]string, 0, len(shas))
		for sha := range shas {
			list = append(list, sha)
		}
		out[name] = list
	}
	return out
}

func (s *InMemoryStore) GetUpdates() []upstreamtypes.ChannelRelease {
	return s.updates
}

func (s *InMemoryStore) SetUpdates(updates []upstreamtypes.ChannelRelease) {
	s.updates = updates
}
