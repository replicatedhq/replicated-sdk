include Makefile.build.mk

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
