#!/usr/bin/env bash

set -euo pipefail

PACT_CLI_VERSION="${PACT_CLI_VERSION:-v2.5.7}"

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    pact_platform="linux-x86_64"
    ;;
  Linux-aarch64|Linux-arm64)
    pact_platform="linux-arm64"
    ;;
  *)
    echo "unsupported platform: $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

version_without_v="${PACT_CLI_VERSION#v}"
install_root="${RUNNER_TEMP:-$PWD/.tmp}/pact-cli"
archive_name="pact-${version_without_v}-${pact_platform}.tar.gz"
archive_path="${install_root}/${archive_name}"
download_url="https://github.com/pact-foundation/pact-standalone/releases/download/${PACT_CLI_VERSION}/${archive_name}"

mkdir -p "${install_root}"

curl -fsSL "${download_url}" -o "${archive_path}"
tar -xzf "${archive_path}" -C "${install_root}"
rm -f "${archive_path}"

bin_dir="${install_root}/pact/bin"

if [ ! -x "${bin_dir}/pact" ]; then
  echo "pact binary not found after install at ${bin_dir}" >&2
  exit 1
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "${bin_dir}" >> "${GITHUB_PATH}"
fi

echo "Installed Pact ${PACT_CLI_VERSION} to ${bin_dir}"
