FROM golang:1.20 as builder

ENV PROJECTPATH=/go/src/github.com/replicatedhq/kots-sdk
WORKDIR $PROJECTPATH

COPY Makefile.build.mk ./
COPY Makefile ./
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY pkg ./pkg

ARG git_tag
ENV GIT_TAG=${git_tag}

RUN make build && mv ./bin/kots-sdk /kots-sdk

FROM golang:1.20

RUN apt-get update && apt-get install -y --no-install-recommends curl gnupg2 ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Setup user
RUN useradd -c 'kots-sdk user' -m -d /home/kots-sdk -s /bin/bash -u 1001 kots-sdk
USER kots-sdk
ENV HOME /home/kots-sdk

COPY --from=builder --chown=kots-sdk:kots-sdk /kots-sdk /kots-sdk

WORKDIR /

EXPOSE 3000
ENTRYPOINT ["/kots-sdk"]
CMD ["api"]
