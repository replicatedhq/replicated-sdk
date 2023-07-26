FROM golang:1.20 as builder

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

FROM golang:1.20

RUN apt-get update && apt-get install -y --no-install-recommends curl gnupg2 ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Setup user
RUN useradd -c 'replicated-sdk user' -m -d /home/replicated-sdk -s /bin/bash -u 1001 replicated-sdk
USER replicated-sdk
ENV HOME /home/replicated-sdk

COPY --from=builder --chown=replicated-sdk:replicated-sdk /replicated-sdk /replicated-sdk

WORKDIR /

EXPOSE 3000
ENTRYPOINT ["/replicated-sdk"]
CMD ["api"]
