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

RUN make build && mv ./bin/replicated /replicated

FROM golang:1.20

RUN apt-get update && apt-get install -y --no-install-recommends curl gnupg2 ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Setup user
RUN useradd -c 'replicated user' -m -d /home/replicated -s /bin/bash -u 1001 replicated
USER replicated
ENV HOME /home/replicated

COPY --from=builder --chown=replicated:replicated /replicated /replicated

WORKDIR /

EXPOSE 3000
ENTRYPOINT ["/replicated"]
CMD ["api"]
