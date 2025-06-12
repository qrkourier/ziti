# this Dockerfile builds docker.io/openziti/ziti-cli:{version}-fips

# get kubectl CLI from a source with Docker Content Trust (DCT)
# FIXME: require DCT at build time
FROM bitnami/kubectl:1.33 AS bitnami-kubectl

# FIXME: This repo requires terms acceptance and is only available on registry.redhat.io.
# FROM registry.access.redhat.com/openshift4/ose-cli AS openshift-cli

FROM ubuntu:23.04
# This build stage grabs artifacts that are copied into the final image.
# It uses the same base as the final image to maximize docker cache hits.

ARG ARTIFACTS_DIR=./release
ARG DOCKER_BUILD_DIR=./dist/docker-images/ziti-cli
# e.g. arm64
ARG TARGETARCH
# e.g. linux
ARG TARGETOS

ARG ZUID=2171
ARG ZGID=2171

ARG HOME=/home/ziggy

LABEL name="openziti/ziti-cli" \
      maintainer="developers@openziti.org" \
      vendor="NetFoundry" \
      summary="Run the OpenZiti CLI" \
      description="Run the OpenZiti CLI"

USER root

# Switch to old-releases.ubuntu.com for legacy Ubuntu 23.04
RUN sed -Ei 's/(security|archive)\.ubuntu\.com/old-releases.ubuntu.com/g' /etc/apt/sources.list

### install packages
RUN   apt-get update \
      && DEBIAN_FRONTEND=noninteractive apt-get install \
            -y --no-install-recommends \
            bash-completion \
            findutils \
            hostname \
            jq \
            less \
            python3 \
            python3-pip \
            tar \
            vim-tiny \
      && rm -rf /var/lib/apt/lists/*

### install Kubernetes CLI
COPY --from=bitnami-kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/

RUN mkdir -p -m0755 /licenses
COPY ./LICENSE /licenses/apache.txt

RUN groupadd --gid ${ZGID} ziggy \
      && useradd --uid ${ZUID} --gid ${ZGID} --system --home-dir ${HOME} --shell /bin/bash ziggy \
      && mkdir -p ${HOME} \
      && chown -R ${ZUID}:${ZGID} ${HOME} \
      && chmod -R g+rwX ${HOME}

RUN mkdir -p /usr/local/bin /etc/bash_completion.d
COPY --chmod=0755 ${ARTIFACTS_DIR}/${TARGETARCH}/${TARGETOS}/ziti-fips /usr/local/bin/ziti

RUN /usr/local/bin/ziti completion bash > /etc/bash_completion.d/ziti_cli

USER ziggy
ENV HOME=${HOME}
WORKDIR ${HOME}
COPY ${DOCKER_BUILD_DIR}/bashrc ${HOME}/.bashrc

ENTRYPOINT [ "ziti" ]
