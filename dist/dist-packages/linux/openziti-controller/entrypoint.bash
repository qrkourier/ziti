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
# Both paths source bootstrap.bash for function definitions (issueLeafCerts, etc.)
# but the 'check' path populates variables from config.yml — the authoritative
# source of truth after bootstrap — rather than from env files or saved answers.
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
# loadConfigVars: populate ZITI_* variables from config.yml so that
# issueLeafCerts() can be called without env files or saved answers.
#
# Parses:
#   identity.{cert,server_cert,key,ca} → derive PKI_ROOT, INTERMEDIATE, SERVER, CLIENT, CA file names
#   ctrl.options.advertiseAddress      → ZITI_CTRL_ADVERTISED_ADDRESS
#   existing server cert SPIFFE SAN    → ZITI_CLUSTER_NODE_NAME
##############################################################################
loadConfigVars() {
  local _config_file="$1"

  # Parse identity paths
  local _server_cert_path _client_cert_path _key_path _ca_cert_path
  _server_cert_path="$(awk '/^identity:/{f=1} f && /^[[:space:]]+server_cert:/{gsub(/^[[:space:]]+server_cert:[[:space:]]*"?|"?[[:space:]]*$/,""); print; exit}' "${_config_file}")"
  _client_cert_path="$(awk '/^identity:/{f=1} f && /^[[:space:]]+cert:/{gsub(/^[[:space:]]+cert:[[:space:]]*"?|"?[[:space:]]*$/,""); print; exit}' "${_config_file}")"
  _ca_cert_path="$(awk '/^identity:/{f=1} f && /^[[:space:]]+ca:/{gsub(/^[[:space:]]+ca:[[:space:]]*"?|"?[[:space:]]*$/,""); print; exit}' "${_config_file}")"

  if [[ -z "${_server_cert_path}" || -z "${_client_cert_path}" || -z "${_ca_cert_path}" ]]; then
    echo "ERROR: cannot parse identity paths from ${_config_file}" >&2
    return 1
  fi

  # Derive PKI structure from identity paths:
  #   server_cert: pki/<intermediate>/certs/<server>.chain.pem
  #   cert:        pki/<intermediate>/certs/<client>.chain.pem
  #   ca:          pki/<ca>/certs/<ca>.cert
  local _certs_dir _intermediate_dir
  _certs_dir="$(dirname "${_server_cert_path}")"
  _intermediate_dir="$(dirname "${_certs_dir}")"

  ZITI_PKI_ROOT="$(dirname "${_intermediate_dir}")"
  ZITI_INTERMEDIATE_FILE="$(basename "${_intermediate_dir}")"
  ZITI_SERVER_FILE="$(basename "${_server_cert_path}" .chain.pem)"
  ZITI_CLIENT_FILE="$(basename "${_client_cert_path}" .chain.pem)"
  # shellcheck disable=SC2034  # used by issueLeafCerts() from bootstrap.bash
  ZITI_CA_FILE="$(basename "$(basename "${_ca_cert_path}" .cert)")"

  # Parse advertised address: "tls:<host>:<port>" → host
  local _adv_raw
  _adv_raw="$(awk '/^ctrl:/{c=1} c && /advertiseAddress:/{gsub(/^.*advertiseAddress:[[:space:]]*"?|"?[[:space:]]*$/,""); print; exit}' "${_config_file}")"
  if [[ -n "${_adv_raw}" ]]; then
    local _stripped="${_adv_raw#tls:}"
    ZITI_CTRL_ADVERTISED_ADDRESS="${_stripped%:*}"
  fi

  # Extract cluster node name from existing server cert's SPIFFE URI SAN.
  # The SPIFFE path is "controller/<node_name>".
  if [[ -s "${_server_cert_path}" ]]; then
    local _spiffe_path
    _spiffe_path="$(openssl x509 -in "${_server_cert_path}" -noout -ext subjectAltName 2>/dev/null \
      | grep -oP 'URI:spiffe://[^/]+/\Kcontroller/[^\s,]+' || true)"
    if [[ "${_spiffe_path}" == controller/* ]]; then
      ZITI_CLUSTER_NODE_NAME="${_spiffe_path#controller/}"
      echo "DEBUG: parsed cluster node name from cert SPIFFE SAN: ${ZITI_CLUSTER_NODE_NAME}" >&3
    fi
  fi

  echo "DEBUG: loadConfigVars: PKI_ROOT=${ZITI_PKI_ROOT} INTERMEDIATE=${ZITI_INTERMEDIATE_FILE} SERVER=${ZITI_SERVER_FILE} CLIENT=${ZITI_CLIENT_FILE} ADDRESS=${ZITI_CTRL_ADVERTISED_ADDRESS:-}" >&3
}

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

  # renew leaf certs if enabled — parse config.yml for all needed values
  if [[ "${ZITI_BOOTSTRAP:-}" == true && "${ZITI_BOOTSTRAP_PKI:-}" == true ]]; then
    # load service.env for ZITI_AUTO_RENEW_CERTS (and ZITI_BOOTSTRAP_* flags)
    loadEnvFiles /opt/openziti/etc/controller/service.env
    if [[ "${ZITI_AUTO_RENEW_CERTS:-true}" == true ]]; then
      loadConfigVars "${2}"
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
