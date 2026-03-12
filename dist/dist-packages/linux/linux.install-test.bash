#!/usr/bin/env bash

# Test fresh install of OpenZiti Linux controller and router packages.
#
# Local:  sudo -i bash /path/to/linux.install-test.bash
# CI:     runs as root with go, nfpm, etc. already on PATH

set -o errexit
set -o nounset
set -o pipefail
set -o errtrace
set -o xtrace

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deployments-test-lib.bash
source "${SCRIPT_DIR}/deployments-test-lib.bash"

# Ensure non-zero exit on any failure and always clean up.
# The ERR trap dumps service diagnostics so failures are never silent.
_exit_code=0
_in_err_handler=0
_err_handler() {
  _exit_code=$?
  # Guard against re-entrancy (diagnostics commands may also fail)
  if (( _in_err_handler )); then return; fi
  _in_err_handler=1
  log_error "FAILED at line ${LINENO}: ${BASH_COMMAND} (exit ${_exit_code})"
  # Redirect to stderr so diagnostics don't pollute command substitution stdout
  # (errtrace causes this handler to fire inside $() subshells)
  dump_service_diagnostics ziti-controller.service >&2
  dump_service_diagnostics ziti-router.service >&2
}
trap '_err_handler' ERR
trap 'cleanup_all; exit $_exit_code' EXIT

BASEDIR="${SCRIPT_DIR}"
REPOROOT="$(cd "${BASEDIR}/../../.." && pwd)"
cd "${REPOROOT}"

declare -a BINS=(grep go nc nfpm curl jq unzip)
for BIN in "${BINS[@]}"; do
    check_command "$BIN"
done

: "${ZITI_GO_VERSION:=$(grep -E '^go [0-9]+\.[0-9]*' "./go.mod" | cut -d " " -f2)}"
: "${ZITI_PWD:=$(generate_password)}"
: "${TMPDIR:=$(mktemp -d)}"
: "${ZITI_CTRL_ADVERTISED_ADDRESS:="ziti-controller1.127.0.0.1.sslip.io"}"
: "${ZITI_CTRL_ADVERTISED_PORT:="1281"}"
: "${ZITI_BOOTSTRAP:=true}"
: "${ZITI_BOOTSTRAP_CLUSTER:=true}"
: "${ZITI_BOOTSTRAP_CONSOLE:=true}"
: "${ZITI_CLUSTER_NODE_NAME:=${ZITI_CTRL_ADVERTISED_ADDRESS%%.*}}"
: "${ZITI_CLUSTER_TRUST_DOMAIN:=${ZITI_CTRL_ADVERTISED_ADDRESS#*.}}"
: "${ZITI_ROUTER_PORT:="30223"}"
: "${ZITI_ROUTER_NAME:="linux-router1"}"
: "${ZITI_ROUTER_ADVERTISED_ADDRESS:="${ZITI_ROUTER_NAME}.127.0.0.1.sslip.io"}"
: "${ZITI_ENROLL_TOKEN:="${TMPDIR}/${ZITI_ROUTER_NAME}.jwt"}"
: "${ZITI_CONSOLE_LOCATION:="/opt/openziti/share/consoletest"}"
: "${ZITI_USER:="admin"}"

export \
ZITI_GO_VERSION \
ZITI_USER \
ZITI_PWD \
ZITI_CTRL_ADVERTISED_ADDRESS \
ZITI_CTRL_ADVERTISED_PORT \
ZITI_BOOTSTRAP \
ZITI_BOOTSTRAP_CLUSTER \
ZITI_BOOTSTRAP_CONSOLE \
ZITI_CLUSTER_NODE_NAME \
ZITI_CLUSTER_TRUST_DOMAIN \
ZITI_ROUTER_PORT \
ZITI_ROUTER_NAME \
ZITI_ROUTER_ADVERTISED_ADDRESS \
ZITI_ENROLL_TOKEN \
ZITI_CONSOLE_LOCATION

cleanup_all

for PORT in "${ZITI_CTRL_ADVERTISED_PORT}" "${ZITI_ROUTER_PORT}"; do
    check_port_available "${PORT}"
done

build_packages
install_local_debs "${TMPDIR}"

# provide dummy console assets before controller bootstrap so /zac/ is configured and served
sudo mkdir -p "${ZITI_CONSOLE_LOCATION}"
sudo tee "${ZITI_CONSOLE_LOCATION}/index.html" <<< "I am ZAC"
sudo chmod -R +rX "${ZITI_CONSOLE_LOCATION}"

# bootstrap.bash now handles:
# 1. PKI generation
# 2. Config file creation
# 3. Starting the controller service
# 4. Cluster initialization (creating default admin)
log_section "Bootstrapping controller"
DEBUG=1 sudo -E /opt/openziti/etc/controller/bootstrap.bash </dev/null  # closing stdin suppresses prompts

# Verify controller service is running (bootstrap.bash should have started it)
wait_for_service ziti-controller.service 30

# Verify the service user can reach the controller agent
sudo -u ziti-controller "${ZITI_BIN}" agent stats

# Wait for controller port to be reachable
wait_for_port "${ZITI_CTRL_ADVERTISED_ADDRESS}" "${ZITI_CTRL_ADVERTISED_PORT}" 30

# shellcheck disable=SC2140
login_cmd="${ZITI_BIN} edge login ${ZITI_CTRL_ADVERTISED_ADDRESS}:${ZITI_CTRL_ADVERTISED_PORT}"\
" --yes"\
" --username admin"\
" --password ${ZITI_PWD}"
# shellcheck disable=SC2086  # intentional word splitting for retry args
retry 10 3 ${login_cmd}

"${ZITI_BIN}" edge create edge-router "${ZITI_ROUTER_NAME}" -to "${ZITI_ENROLL_TOKEN}"

if [[ -z "${ZITI_ENROLL_TOKEN:-}" || ! -s "${ZITI_ENROLL_TOKEN}" ]]; then
    log_error "router enrollment token not found at ${ZITI_ENROLL_TOKEN:-<unset>}"
    exit 1
fi
ZITI_ENROLL_TOKEN_CONTENT="$(<"${ZITI_ENROLL_TOKEN}")"
if [[ -z "${ZITI_ENROLL_TOKEN_CONTENT}" ]]; then
    log_error "router enrollment token is empty in ${ZITI_ENROLL_TOKEN}"
    exit 1
fi
export ZITI_ENROLL_TOKEN="${ZITI_ENROLL_TOKEN_CONTENT}"

log_section "Bootstrapping router"
ZITI_BOOTSTRAP=true ZITI_BOOTSTRAP_ENROLLMENT=true DEBUG=1 \
    sudo -E /opt/openziti/etc/router/bootstrap.bash </dev/null  # closing stdin suppresses prompts
start_service ziti-router.service
wait_for_service ziti-router.service 20

retry 10 3 bash -c "[[ \$($ZITI_BIN edge list edge-routers -j | jq \".data[0].isOnline\") == \"true\" ]]"
log_info "router is online"

export \
ZITI_CTRL_EDGE_ADVERTISED_ADDRESS=${ZITI_CTRL_ADVERTISED_ADDRESS} \
ZITI_CTRL_EDGE_ADVERTISED_PORT=${ZITI_CTRL_ADVERTISED_PORT}

_test_result=$(go test -v -count=1 -tags="quickstart manual" ./ziti/run/...)

# check for failure modes that don't result in an error exit code
if [[ "${_test_result}" =~ "no tests to run" ]]; then
    log_error "test failed because no tests to run"
    exit 1
fi

# verify console is available
log_section "Verifying console"
curl_cmd="curl -skSfw '%{http_code}\t%{url}\n' -o/dev/null \"https://${ZITI_CTRL_ADVERTISED_ADDRESS}:${ZITI_CTRL_ADVERTISED_PORT}/zac/\""
retry 5 3 eval "${curl_cmd}"
eval "${curl_cmd}"

log_section "All install tests passed"
# cleanup runs via EXIT trap
