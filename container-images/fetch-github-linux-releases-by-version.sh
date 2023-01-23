#!/bin/bash

#
# Copyright 2023 NetFoundry Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -euo pipefail

[[ $# -eq 0 ]] && {
    echo "ERROR: need the base name of the executable to compose URL to Linux release download e.g. \"ziti\"." >&2
    exit 1
}

echo "Fetching from GitHub."
# defaults
: "${GITHUB_BASE_URL:=https://github.com}"
: "${GITHUB_REPO:="openziti/ziti"}"
: "${ZITI_VERSION:="latest"}"

if ! [[ "$ZITI_VERSION" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]];then
    # this script only gets versioned releases, never "latest"
    echo "ERROR: ZITI_VERSION must be a semver" >&2
    exit 1
fi

# ensure version string begins with 'v'
ZITI_VERSION="v${ZITI_VERSION#v}"

# map host architecture/os to directories that we use in GitHub.
# (our artifact directories seem to align with Docker's TARGETARCH and TARGETOS
#  build arguments, which we could rely on if we fully committed to "docker buildx" - see
#  https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope)
HOST_ARCH=$(uname -m)
case "${HOST_ARCH}" in
    "x86_64") ARTIFACT_ARCH="amd64";;
    "armv7l") ARTIFACT_ARCH="arm";;
    "aarch64") ARTIFACT_ARCH="arm64";;
    *) echo "ERROR: Ziti binaries do not exist for architecture ${HOST_ARCH}"; exit 1;;
esac

HOST_OS=$(uname -s)
case "${HOST_OS}" in
    "Linux") ARTIFACT_OS="Linux";;
    *) echo "ERROR: this script gets binaries for the Linux container image, not ${HOST_OS}"; exit 1;;
esac

# for each positional param do try to download an artifact with the same name
for EXE in "${@}"; do
    TARBALL="${EXE}-${ARTIFACT_OS}-${ARTIFACT_ARCH}-${ZITI_VERSION#v}.tar.gz"
    case "${ZITI_VERSION}" in
        "latest") URL="${GITHUB_BASE_URL}/${GITHUB_REPO}/releases/${ZITI_VERSION}/download/${TARBALL}" ;;
        *)        URL="${GITHUB_BASE_URL}/${GITHUB_REPO}/releases/download/${ZITI_VERSION}/${TARBALL}" ;;
    esac
    
    echo "Fetching ${TARBALL} from ${URL}"
    rm -f "${TARBALL}" "${EXE}"
    if { command -v curl > /dev/null; } 2>&1; then
        curl -fLsS -O "${URL}"
    elif { command -v wget > /dev/null; } 2>&1; then
        wget "${URL}"
    else
        echo "ERROR: need one of curl or wget to fetch the artifact." >&2
        exit 1
    fi
    tar -xvf "${TARBALL}"

    if [[ -f "${EXE}" ]]; then 
        chmod 0755 "${EXE}"
    else
        echo "ERROR: missing executable ${EXE}"
        exit 1
    fi
    rm -f "${TARBALL}"
done
