FROM cgr.dev/chainguard/go:1.20 as builder

ENV PROJECTPATH=/go/src/github.com/replicatedhq/replicated-sdk
WORKDIR $PROJECTPATH

COPY Makefile.build.mk ./
COPY Makefile ./
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY pkg ./pkg

ARG git_tag
ENV GIT_TAG=${git_tag}

RUN make build && mv ./bin/replicated-sdk /replicated-sdk

FROM cgr.dev/chainguard/static:latest

USER replicated-sdk

COPY --from=builder --chown=replicated-sdk:replicated-sdk /replicated-sdk /replicated-sdk

WORKDIR /

EXPOSE 3000
ENTRYPOINT ["/replicated-sdk"]
CMD ["api"]
