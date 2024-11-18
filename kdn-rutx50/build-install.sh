#!/usr/bin/env bash
set -eEuo pipefail
set -x

pushd "${BASH_SOURCE[0]%/*}"
wd="${PWD}"
repo="${wd}/.."
out="${repo}/dist/netbird"
local="${repo}"
conn="root@yelk.yelk.lan."

upload() {
  local local="$1" remote="$2"
  if test "$(sha256sum <"${local}")" != "$(ssh "${conn}" "sha256sum <${remote@Q} || :")"; then
    ssh "${conn}" "mkdir -p '${remote%/*}'"
    scp "${local}" "${conn}":"${remote}"
    changed+=("uploaded: ${remote}")
  fi
}

download() {
  local remote="$1" local="$2"
  if test "$(ssh "${conn}" "sha256sum <${remote@Q}")" != "$(sha256sum <"${local}")"; then
    ssh "${conn}" "mkdir -p '${remote%/*}'"
    scp "${conn}":"${remote}" "${local}"
    changed+=("downloaded: ${remote}")
  fi
}

main() {
  changed=()

  git --no-pager log "origin~1..HEAD"

  mkdir -p "${out%/*}"
  go_args=(
    # make the binary reproducible
    -ldflags "-s -w"
  )
  GOARM=6 GOARCH=arm CGO_ENABLED=0 go build "${go_args[@]}" -o "${out}" "${local}/client"
  upload "${out}" "/root/netbird"
  upload "${wd}/sysconfig.sh" /etc/sysconfig/netbird
  upload "${wd}/configure.sh" /root/netbird-configure.sh
  upload "${wd}/config.json" /root/config.json
  download /etc/netbird/config.json "${wd}/config.json"

  if test "${#changed[@]}" -gt 0; then
    ssh "${conn}" /root/netbird-configure.sh
  fi
}

main "$@"
