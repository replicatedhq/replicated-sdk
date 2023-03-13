BUILDFLAGS = -tags='netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp' -installsuffix netgo
TEST_BUILDFLAGS = -tags='testing netgo containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp' -installsuffix netgo

.PHONY: test
test:
	go test $(TEST_BUILDFLAGS) ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: build
build:
	go build ${LDFLAGS} ${GCFLAGS} -v -o bin/kots-sdk $(BUILDFLAGS) ./cmd/kots-sdk

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet $(BUILDFLAGS) ./pkg/... ./cmd/...

.PHONY: build-ttl.sh
build-ttl.sh:
	docker build -t ttl.sh/${USER}/kots-sdk:24h .
	docker push ttl.sh/${USER}/kots-sdk:24h

	make -C chart build-ttl.sh
