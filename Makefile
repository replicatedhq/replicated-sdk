include Makefile.build.mk

.PHONY: test-unit
test-unit:
	go test -v $(TEST_BUILDFLAGS) ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: test-pact
test-pact:
	@echo "Running Pact tests..."
	@if ! command -v pact-mock-service &> /dev/null; then \
		echo "The 'pact-mock-service' command is not found on your PATH. Please install the CLI from the releases page in https://github.com/pact-foundation/pact-ruby-standalone."; \
	else \
		go test -v ./pact/... -tags "$(BUILDTAGS)"; \
	fi

.PHONY: test
test: test-unit test-pact

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
		--environment staging \
		--verbose
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
	docker buildx build .  -t ttl.sh/${USER}/replicated-sdk:24h -f deploy/Dockerfile
	docker push ttl.sh/${USER}/replicated-sdk:24h

	make -C chart build-ttl.sh

.PHONY: mock
mock:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -source=pkg/store/store_interface.go -destination=pkg/store/mock/mock_store.go

.PHONY: scan
scan:
	trivy fs \
		--scanners vuln \
		--exit-code=1 \
		--severity="CRITICAL,HIGH,MEDIUM" \
		--ignore-unfixed \
		./
