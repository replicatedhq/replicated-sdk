include Makefile.build.mk

.PHONY: test
test:
	go test -v $(TEST_BUILDFLAGS) ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: publish-pact
publish-pact:
	pact-broker publish ./pacts \
		--auto-detect-version-properties \
		--consumer-app-version ${PACT_VERSION} \
		--verbose

.PHONY: can-i-deploy
can-i-deploy:
	pact-broker can-i-deploy \
		--pacticipant replicated-sdk \
		--version ${PACT_VERSION} \
		--to-environment production \
		--verbose

.PHONY: record-release
record-release:
	pact-broker record-release \
		--pacticipant replicated-sdk \
		--version ${PACT_VERSION} \
		--environment production \
		--verbose

.PHONY: build
build:
	go build ${LDFLAGS} ${GCFLAGS} -v -o bin/replicated $(BUILDFLAGS) ./cmd/replicated

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet $(BUILDFLAGS) ./pkg/... ./cmd/...

.PHONY: build-ttl.sh
build-ttl.sh:
	docker build -t ttl.sh/${USER}/replicated:24h .
	docker push ttl.sh/${USER}/replicated:24h

	make -C chart build-ttl.sh

.PHONY: mock
mock:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -source=pkg/store/store_interface.go -destination=pkg/store/mock/mock_store.go
