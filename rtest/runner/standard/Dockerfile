FROM docker:29.1.3-cli@sha256:4fa0ee1f3a7e4354c4ea34558b6d4ee32859baf4973d4c8ccc8e7fe3dd730c04 AS docker-cli

FROM golang:1.25-bookworm@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
       ca-certificates git gcc g++ make pkg-config sqlite3 zstd \
    && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1
COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY rtest-job-entrypoint /usr/local/bin/rtest-job-entrypoint
WORKDIR /workspace
