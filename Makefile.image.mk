# Local equivalents of the explicit image/chart commands in GitHub Actions.
LOCAL_USER ?= $(shell id -un)
IMAGE_DIR ?= $(CURDIR)/bin/sdk-image
IMAGE_REPOSITORY ?= $(LOCAL_USER)/replicated-sdk
IMAGE ?= ttl.sh/$(IMAGE_REPOSITORY):24h
CHART_REPOSITORY ?= oci://ttl.sh/$(LOCAL_USER)
CHANNEL_NAME ?= local-$(LOCAL_USER)
ARCHS ?= x86_64
APKO_ARCHS ?= amd64
CLUSTER_TAG ?= replicated-sdk-local=$(shell id -un)
KUBECONFIG ?= $(CURDIR)/bin/sdk-kubeconfig
E2E_BUILD_INFO ?= $(CURDIR)/build-info.json
export KUBECONFIG E2E_BUILD_INFO E2E_DISTRIBUTION E2E_VERSION

.PHONY: build-image
build-image:
	mkdir -p "$(IMAGE_DIR)/sbom"
	melange keygen "$(IMAGE_DIR)/melange.rsa"
	sudo melange build securebuild/package/melange.yaml \
	  --pipeline-dir .github/actions/build-securebuild-package-locally/pipelines \
	  --source-dir "$(CURDIR)" --out-dir "$(IMAGE_DIR)/packages" \
	  --signing-key "$(IMAGE_DIR)/melange.rsa" --arch $(ARCHS)
	sudo chown -R "$$(id -u):$$(id -g)" "$(IMAGE_DIR)/packages"

.PHONY: publish-image
publish-image: build-image
	apko publish securebuild/image/apko.yaml "$(IMAGE)" \
	  --repository-append "$(IMAGE_DIR)/packages" --keyring-append "$(IMAGE_DIR)/melange.rsa.pub" \
	  --arch $(APKO_ARCHS) --cache-dir /tmp/apko-cache --sbom-path "$(IMAGE_DIR)/sbom"

.PHONY: build-test-chart
build-test-chart: publish-image
	rm -rf "$(IMAGE_DIR)/sdk-chart" "$(IMAGE_DIR)/test-chart"
	cp -R chart "$(IMAGE_DIR)/sdk-chart"
	sed -e 's/registry: proxy.replicated.com/registry: ttl.sh/' -e 's|repository: "library/replicated-sdk-image"|repository: "$(IMAGE_REPOSITORY)"|' -e 's/tag: "1.0.0"/tag: "24h"/' chart/values.yaml > "$(IMAGE_DIR)/sdk-chart/values.yaml"
	helm package "$(IMAGE_DIR)/sdk-chart" --version 0.0.0 --app-version 0.0.0 --destination chart
	helm push chart/replicated-0.0.0.tgz "$(CHART_REPOSITORY)"
	cp -R test-chart "$(IMAGE_DIR)/test-chart"
	sed -e 's|CHART_REPOSITORY|$(CHART_REPOSITORY)|' -e 's/version: 1.0.0/version: 0.0.0/' e2e/chart-dependency.yaml >> "$(IMAGE_DIR)/test-chart/Chart.yaml"
	helm dependency update "$(IMAGE_DIR)/test-chart"
	helm package "$(IMAGE_DIR)/test-chart" --destination "$(IMAGE_DIR)"

.PHONY: build-ttl.sh
build-ttl.sh: build-test-chart
	helm push "$(IMAGE_DIR)/test-chart-0.1.0.tgz" "$(CHART_REPOSITORY)"

.PHONY: build-e2e
build-e2e: build-test-chart
	mkdir -p "$(IMAGE_DIR)/release"
	cp e2e/manifests/*.yaml "$(IMAGE_DIR)/release/"
	cp "$(IMAGE_DIR)/test-chart-0.1.0.tgz" "$(IMAGE_DIR)/release/"
	replicated channel create --app replicated-sdk-e2e --name "$(CHANNEL_NAME)"
	replicated release create --app replicated-sdk-e2e --version 0.1.0 --promote "$(CHANNEL_NAME)" --yaml-dir "$(IMAGE_DIR)/release"
	replicated customer create --app replicated-sdk-e2e --kots-install=false --name test-customer --channel "$(CHANNEL_NAME)" --email test-customer@replicated.com --output json > "$(IMAGE_DIR)/customer.json"
	replicated cluster versions --output json | jq '[.[] | select(.short_name=="gke" or .short_name=="eks" or .short_name=="openshift") | {Distribution:.short_name,Version:.versions[-1]}]' > "$(IMAGE_DIR)/distributions.json"
	jq -n \
	  --arg repository "$(IMAGE_REPOSITORY)" \
	  --arg chart "$(CHART_REPOSITORY)" \
	  --arg channel "$(CHANNEL_NAME)" \
	  --arg digest "$$(crane digest --platform linux/amd64 "$(IMAGE)")" \
	  --arg appID "$$(replicated app ls | awk '$$(NF-1)=="replicated-sdk-e2e" {print $$1}')" \
	  --slurpfile customer "$(IMAGE_DIR)/customer.json" \
	  --slurpfile distributions "$(IMAGE_DIR)/distributions.json" \
	  '{imageRegistry:"ttl.sh",imageRepository:$$repository,imageTag:"24h",imageDigest:$$digest,chart:$$chart,channelSlug:$$channel,appID:$$appID,customerID:$$customer[0].id,licenseID:$$customer[0].installationId,distributions:$$distributions[0]}' > build-info.json

.PHONY: cluster-e2e
cluster-e2e:
	mkdir -p bin
	replicated cluster create --distribution "$(E2E_DISTRIBUTION)" --version "$(E2E_VERSION)" --tag "$(CLUSTER_TAG)" --wait 15m --output json > bin/cluster.json
	replicated cluster kubeconfig "$$(jq -r '.id' bin/cluster.json)" --stdout > "$(KUBECONFIG)"

.PHONY: test-e2e
test-e2e:
	helm registry login registry.replicated.com --username test-customer@replicated.com --password "$$(jq -r '.licenseID' "$(E2E_BUILD_INFO)")"
	SDK_E2E=1 go test ./e2e -run '^TestSDK$$' -count=1 -v -timeout=90m

.PHONY: cleanup-e2e
cleanup-e2e:
	replicated cluster rm --tag "$(CLUSTER_TAG)"
