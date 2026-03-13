#!/usr/bin/env bash
#
# Entrypoint for the OpenZiti Controller systemd service and Docker container.
#
# Modes:
#   check config.yml  — preflight: verify config exists, database dir is writable,
#                        and optionally renew leaf certs (ExecStartPre on Linux)
#   run config.yml    — start the controller; in Docker with ZITI_BOOTSTRAP=true,
#                        run bootstrap() first to generate PKI/config/database
#
# Both paths source bootstrap.bash for function definitions (issueLeafCerts, etc.).
# The 'check' path loads service.env (feature flags) and state.env (deployment-specific
# answers: PKI paths, node name, advertised address, etc.) so cert renewal uses the
# actual deployment values rather than relying on defaults.
#
# usage:
#   entrypoint.bash run config.yml
#   entrypoint.bash check config.yml

set -o errexit
set -o nounset
set -o pipefail

# discard debug unless DEBUG
: "${DEBUG:=0}"
if (( DEBUG )); then
  exec 3>&1
  set -o xtrace
else
  exec 3>/dev/null
fi

# default unless args
if ! (( $# )) || [[ "${1}" == run && -z "${2:-}" ]]; then
  set -- run config.yml
fi

# Source bootstrap.bash for function definitions and PKI defaults.
# The source guard in bootstrap.bash (BASH_SOURCE != $0) ensures only functions
# and defaults are loaded — no prompts, no bootstrap execution.
# shellcheck disable=SC1090
source "${ZITI_CTRL_BOOTSTRAP_BASH:-/opt/openziti/etc/controller/bootstrap.bash}"

# Clear bootstrap's exit trap — the entrypoint manages its own lifecycle
trap - EXIT SIGINT SIGTERM

##############################################################################
# Main dispatch
##############################################################################

if [[ "${1}" =~ check ]]; then
  # --- Preflight check (Linux ExecStartPre) ---
  if [[ ! -s "${2}" ]]; then
    echo "ERROR: ${2} does not exist" >&2
    hintLinuxBootstrap "${PWD}"
    exit 1
  fi

  # check writability of database directory if the config defines one
  _data_dir="$(dataDir "${2}")"
  if [[ -n "${_data_dir}" && ! -w "${_data_dir}" ]]; then
    echo "ERROR: database directory '${_data_dir}' is not writable" >&2
    hintLinuxBootstrap "${PWD}"
    exit 1
  fi

  # Renew leaf certs if enabled.
  # service.env has feature flags (ZITI_BOOTSTRAP, ZITI_AUTO_RENEW_CERTS, etc.).
  # state.env has deployment-specific answers from bootstrap (ZITI_CLUSTER_NODE_NAME,
  # ZITI_CTRL_ADVERTISED_ADDRESS, ZITI_PKI_ROOT, ZITI_INTERMEDIATE_FILE, etc.).
  # state.env is loaded second so deployment values override any defaults.
  if [[ "${ZITI_BOOTSTRAP:-}" == true && "${ZITI_BOOTSTRAP_PKI:-}" == true ]]; then
    loadEnvFiles /opt/openziti/etc/controller/service.env /var/lib/ziti-controller/state.env
    if [[ "${ZITI_AUTO_RENEW_CERTS:-true}" == true ]]; then
      echo "DEBUG: issueLeafCerts: NODE_NAME=${ZITI_CLUSTER_NODE_NAME:-} ADDRESS=${ZITI_CTRL_ADVERTISED_ADDRESS:-} PKI_ROOT=${ZITI_PKI_ROOT:-} INTERMEDIATE=${ZITI_INTERMEDIATE_FILE:-}" >&3
      issueLeafCerts
    fi
  fi
  exit 0

elif [[ "${ZITI_BOOTSTRAP:-}" == true && "${1}" =~ run ]]; then
  # --- Docker/container bootstrap path ---
  # bootstrap() uses env vars for first-run setup (PKI, config, database).
  # On subsequent container restarts, config.yml already exists and bootstrap()
  # detects that — it only renews certs if ZITI_AUTO_RENEW_CERTS=true.
  bootstrap "${2}"

  # If cluster initialization is needed, start controller in background, init, then wait
  if [[ "${ZITI_BOOTSTRAP_CLUSTER:-}" == true || "${ZITI_BOOTSTRAP_DATABASE:-}" == true ]] \
     && [[ -n "${ZITI_PWD:-}" ]]; then
    echo "INFO: starting controller in background for cluster initialization"
    # shellcheck disable=SC2068
    ziti controller ${@} &
    _ctrl_pid=$!

    if waitForAgent "" 30 1; then
      clusterInit ""
    fi

    wait "${_ctrl_pid}"
  else
    # shellcheck disable=SC2068
    exec ziti controller ${@}
  fi

else
  # --- Normal run (no bootstrap) ---
  # shellcheck disable=SC2068
  exec ziti controller ${@}
fi
