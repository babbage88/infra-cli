#!/usr/bin/env bash
set -euo pipefail

# Verification helper for the recently modified commands.
# Edit the variables in the sections below, then run:
#   bash scripts/verify_modified_commands.sh
#
# If you prefer `go run`, change this to:
#   INFRACTL_BIN=(go run .)
INFRACTL_BIN=(dist/infractl)

# Set to false to print commands without executing them.
EXECUTE=true

# Toggle individual checks.
RUN_DEPLOY_APP_SYSTEMD=false
RUN_STORAGE_S3_DEPLOY_GARAGE_NODE=true
RUN_DATABASE_VALKEY_NEW=true
RUN_DATABASE_NEWAPPDB=true
RUN_PROXMOX_LXC_CREATE=false

# Shared SSH defaults. Override per command below when needed.
COMMON_SSH_USER="root"
COMMON_SSH_KEY="${HOME}/.ssh/trahan_ed25519"
COMMON_SSH_PASSPHRASE=""
COMMON_SSH_PORT="22"
COMMON_SSH_USE_AGENT="true"

# deploy app-systemd
DEPLOY_SSH_HOST="rockydev1"
DEPLOY_SSH_USER="${COMMON_SSH_USER}"
DEPLOY_SSH_KEY="${COMMON_SSH_KEY}"
DEPLOY_SSH_PASSPHRASE="${COMMON_SSH_PASSPHRASE}"
DEPLOY_SSH_PORT="${COMMON_SSH_PORT}"
DEPLOY_SSH_USE_AGENT="${COMMON_SSH_USE_AGENT}"
DEPLOY_APP_NAME="verify-systemd-app"
DEPLOY_SOURCE_BIN="./dist/verify-systemd-app"
DEPLOY_SERVICE_UID="8888"
DEPLOY_INSTALL_DIR="/opt/verify-systemd-app"
DEPLOY_ENV_VARS="APP_ENV=staging,VERIFY_RUN=1"

# storage s3 deploy-garage-node
GARAGE_SSH_HOST="rockydev1"
GARAGE_SSH_USER=$USER
GARAGE_SSH_KEY="${COMMON_SSH_KEY}"
GARAGE_SSH_PASSPHRASE="${COMMON_SSH_PASSPHRASE}"
GARAGE_SSH_PORT="${COMMON_SSH_PORT}"
GARAGE_SSH_USE_AGENT="${COMMON_SSH_USE_AGENT}"
GARAGE_RPC_PUBLIC_ADDR=""
GARAGE_REPLICATION_FACTOR="1"

# database valkey new
VALKEY_SSH_HOST="10.2.10.248"
VALKEY_SSH_USER="${COMMON_SSH_USER}"
VALKEY_SSH_KEY="${COMMON_SSH_KEY}"
VALKEY_SSH_PASSPHRASE="${COMMON_SSH_PASSPHRASE}"
VALKEY_SSH_PORT="${COMMON_SSH_PORT}"
VALKEY_SSH_USE_AGENT="${COMMON_SSH_USE_AGENT}"
VALKEY_USERNAME="verify-user"
VALKEY_PASSWORD="verify-password"
VALKEY_BIND="0.0.0.0"
VALKEY_PORT="6379"

# database new-appdb
APPDB_SSH_HOST="10.2.10.248"
APPDB_SSH_USER="${COMMON_SSH_USER}"
APPDB_SSH_KEY="${COMMON_SSH_KEY}"
APPDB_SSH_PASSPHRASE="${COMMON_SSH_PASSPHRASE}"
APPDB_SSH_PORT="${COMMON_SSH_PORT}"
APPDB_SSH_USE_AGENT="${COMMON_SSH_USE_AGENT}"
APPDB_NAME="verify_appdb"
APPDB_USER="verify_app"
APPDB_PASSWORD="verify-password"
APPDB_POSTGRES_USER="postgres"
APPDB_POSTGRES_PASSWORD=""
APPDB_POSTGRES_PORT="5432"
APPDB_SETUP_REMOTE_POSTGRES="false"

# proxmox lxc create
PVE_SSH_HOST="proxmox3"
PVE_SSH_USER="root"
PVE_SSH_KEY="${COMMON_SSH_KEY}"
PVE_SSH_PASSPHRASE="${COMMON_SSH_PASSPHRASE}"
PVE_SSH_PORT="${COMMON_SSH_PORT}"
PVE_SSH_USE_AGENT="${COMMON_SSH_USE_AGENT}"
PROXMOX_API_URL="https://pve01.example.internal:8006"
PROXMOX_API_TOKEN="root@pam!infractl-cli=replace-me"
PROXMOX_PVE_NODE="pve01"
PROXMOX_LXC_VMID="9191"
PROXMOX_LXC_HOSTNAME="verify-lxc-01"
PROXMOX_LXC_OSTEMPLATE="local:vztmpl/rockylinux-10-default_20251001_amd64.tar.xz"
PROXMOX_LXC_STORAGE="local-lvm"
PROXMOX_LXC_ROOTFS_SIZE="9"
PROXMOX_LXC_NET0="name=eth0,bridge=vmbr0,ip=dhcp,type=veth"
PROXMOX_LXC_PUBLIC_KEY_PATH="${HOME}/.ssh/id_ed25519.pub"
PROXMOX_LXC_VERIFY_SSH_USER="root"
PROXMOX_LXC_VERIFY_SSH_PORT="22"

require_var() {
  local name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    echo "Missing required value: ${name}" >&2
    exit 1
  fi
}

append_root_ssh_flags() {
  local host="$1"
  local user="$2"
  local key="$3"
  local passphrase="$4"
  local port="$5"
  local use_agent="$6"

  ROOT_SSH_FLAGS=()
  [[ -n "${host}" ]] && ROOT_SSH_FLAGS+=(--ssh-remote-host "${host}")
  [[ -n "${user}" ]] && ROOT_SSH_FLAGS+=(--ssh-remote-user "${user}")
  if [[ -n "${key}" ]]; then
    if [[ -f "${key}" ]]; then
      ROOT_SSH_FLAGS+=(--ssh-key "${key}")
    elif [[ "${use_agent}" == "true" ]]; then
      echo "warning: skipping missing --ssh-key ${key} because ssh-agent mode is enabled" >&2
    else
      echo "error: configured ssh key does not exist: ${key}" >&2
      exit 1
    fi
  fi
  [[ -n "${passphrase}" ]] && ROOT_SSH_FLAGS+=(--ssh-passphrase "${passphrase}")
  [[ -n "${port}" ]] && ROOT_SSH_FLAGS+=(--ssh-port "${port}")
  [[ "${use_agent}" == "true" ]] && ROOT_SSH_FLAGS+=(--ssh-use-agent)
}

run_cmd() {
  printf '\n==> '
  printf '%q ' "$@"
  printf '\n'

  if [[ "${EXECUTE}" == "true" ]]; then
    "$@"
  fi
}

run_deploy_app_systemd() {
  require_var "DEPLOY_SSH_HOST" "${DEPLOY_SSH_HOST}"
  require_var "DEPLOY_APP_NAME" "${DEPLOY_APP_NAME}"
  require_var "DEPLOY_SOURCE_BIN" "${DEPLOY_SOURCE_BIN}"

  local -a cmd=("${INFRACTL_BIN[@]}" deploy app-systemd
    --app-name "${DEPLOY_APP_NAME}"
    --source-bin "${DEPLOY_SOURCE_BIN}"
    --service-uid "${DEPLOY_SERVICE_UID}"
    --install-dir "${DEPLOY_INSTALL_DIR}"
  )
  [[ -n "${DEPLOY_ENV_VARS}" ]] && cmd+=(--env-vars "${DEPLOY_ENV_VARS}")
  append_root_ssh_flags "${DEPLOY_SSH_HOST}" "${DEPLOY_SSH_USER}" "${DEPLOY_SSH_KEY}" "${DEPLOY_SSH_PASSPHRASE}" "${DEPLOY_SSH_PORT}" "${DEPLOY_SSH_USE_AGENT}"
  cmd+=("${ROOT_SSH_FLAGS[@]}")
  run_cmd "${cmd[@]}"
}

run_storage_s3_deploy_garage_node() {
  require_var "GARAGE_SSH_HOST" "${GARAGE_SSH_HOST}"

  local -a cmd=("${INFRACTL_BIN[@]}" storage s3 deploy-garage-node
    --garage-replication-factor "${GARAGE_REPLICATION_FACTOR}"
  )
  [[ -n "${GARAGE_RPC_PUBLIC_ADDR}" ]] && cmd+=(--garage-rpc-public-addr "${GARAGE_RPC_PUBLIC_ADDR}")
  append_root_ssh_flags "${GARAGE_SSH_HOST}" "${GARAGE_SSH_USER}" "${GARAGE_SSH_KEY}" "${GARAGE_SSH_PASSPHRASE}" "${GARAGE_SSH_PORT}" "${GARAGE_SSH_USE_AGENT}"
  cmd+=("${ROOT_SSH_FLAGS[@]}")
  run_cmd "${cmd[@]}"
}

run_database_valkey_new() {
  require_var "VALKEY_SSH_HOST" "${VALKEY_SSH_HOST}"
  require_var "VALKEY_USERNAME" "${VALKEY_USERNAME}"
  require_var "VALKEY_PASSWORD" "${VALKEY_PASSWORD}"

  local -a cmd=("${INFRACTL_BIN[@]}" database valkey new
    --username "${VALKEY_USERNAME}"
    --password "${VALKEY_PASSWORD}"
    --bind "${VALKEY_BIND}"
    --port "${VALKEY_PORT}"
  )
  append_root_ssh_flags "${VALKEY_SSH_HOST}" "${VALKEY_SSH_USER}" "${VALKEY_SSH_KEY}" "${VALKEY_SSH_PASSPHRASE}" "${VALKEY_SSH_PORT}" "${VALKEY_SSH_USE_AGENT}"
  cmd+=("${ROOT_SSH_FLAGS[@]}")
  run_cmd "${cmd[@]}"
}

run_database_newappdb() {
  require_var "APPDB_SSH_HOST" "${APPDB_SSH_HOST}"
  require_var "APPDB_NAME" "${APPDB_NAME}"
  require_var "APPDB_USER" "${APPDB_USER}"
  require_var "APPDB_PASSWORD" "${APPDB_PASSWORD}"

  local -a cmd=("${INFRACTL_BIN[@]}" database new-appdb
    --connect-ssh
    --db-name "${APPDB_NAME}"
    --db-user "${APPDB_USER}"
    --db-password "${APPDB_PASSWORD}"
    --create-db
    --postgres-user "${APPDB_POSTGRES_USER}"
    --postgres-password "${APPDB_POSTGRES_PASSWORD}"
    --postgres-port "${APPDB_POSTGRES_PORT}"
  )
  if [[ "${APPDB_SETUP_REMOTE_POSTGRES}" == "true" ]]; then
    cmd+=(--setup-remote-postgres)
  fi
  append_root_ssh_flags "${APPDB_SSH_HOST}" "${APPDB_SSH_USER}" "${APPDB_SSH_KEY}" "${APPDB_SSH_PASSPHRASE}" "${APPDB_SSH_PORT}" "${APPDB_SSH_USE_AGENT}"
  cmd+=("${ROOT_SSH_FLAGS[@]}")
  run_cmd "${cmd[@]}"
}

run_proxmox_lxc_create() {
  require_var "PROXMOX_API_URL" "${PROXMOX_API_URL}"
  require_var "PROXMOX_API_TOKEN" "${PROXMOX_API_TOKEN}"
  require_var "PROXMOX_PVE_NODE" "${PROXMOX_PVE_NODE}"
  require_var "PROXMOX_LXC_VMID" "${PROXMOX_LXC_VMID}"
  require_var "PROXMOX_LXC_HOSTNAME" "${PROXMOX_LXC_HOSTNAME}"
  require_var "PROXMOX_LXC_OSTEMPLATE" "${PROXMOX_LXC_OSTEMPLATE}"
  require_var "PROXMOX_LXC_STORAGE" "${PROXMOX_LXC_STORAGE}"
  require_var "PROXMOX_LXC_PUBLIC_KEY_PATH" "${PROXMOX_LXC_PUBLIC_KEY_PATH}"

  local ssh_public_key
  ssh_public_key="$(<"${PROXMOX_LXC_PUBLIC_KEY_PATH}")"
  require_var "ssh public key contents" "${ssh_public_key}"

  local -a cmd=("${INFRACTL_BIN[@]}" proxmox lxc create
    --host-url "${PROXMOX_API_URL}"
    --api-token "${PROXMOX_API_TOKEN}"
    --pve-node "${PROXMOX_PVE_NODE}"
    --vmid "${PROXMOX_LXC_VMID}"
    --lxc-hostname "${PROXMOX_LXC_HOSTNAME}"
    --ostemplate "${PROXMOX_LXC_OSTEMPLATE}"
    --storage "${PROXMOX_LXC_STORAGE}"
    --rootfs-size "${PROXMOX_LXC_ROOTFS_SIZE}"
    --net0 "${PROXMOX_LXC_NET0}"
    --ssh-public-keys "${ssh_public_key}"
    --verify
    --verify-ssh-user "${PROXMOX_LXC_VERIFY_SSH_USER}"
    --verify-ssh-port "${PROXMOX_LXC_VERIFY_SSH_PORT}"
  )
  append_root_ssh_flags "${PVE_SSH_HOST}" "${PVE_SSH_USER}" "${PVE_SSH_KEY}" "${PVE_SSH_PASSPHRASE}" "${PVE_SSH_PORT}" "${PVE_SSH_USE_AGENT}"
  cmd+=("${ROOT_SSH_FLAGS[@]}")
  run_cmd "${cmd[@]}"
}

main() {
  [[ "${RUN_DEPLOY_APP_SYSTEMD}" == "true" ]] && run_deploy_app_systemd
  [[ "${RUN_STORAGE_S3_DEPLOY_GARAGE_NODE}" == "true" ]] && run_storage_s3_deploy_garage_node
  [[ "${RUN_DATABASE_VALKEY_NEW}" == "true" ]] && run_database_valkey_new
  [[ "${RUN_DATABASE_NEWAPPDB}" == "true" ]] && run_database_newappdb
  [[ "${RUN_PROXMOX_LXC_CREATE}" == "true" ]] && run_proxmox_lxc_create
}

main "$@"
