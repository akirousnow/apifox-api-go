#!/usr/bin/env bash
# Cross-compile apifox-api for the six supported platforms.
# Release builds pin the Go toolchain to 1.26.5 when available.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
OUT_DIR="${OUT_DIR:-${ROOT_DIR}/dist/release}"
MODULE_PATH="github.com/akirousnow/apifox-api-go"
LDFLAGS="-s -w -X ${MODULE_PATH}/internal/buildinfo.Version=${VERSION} -X ${MODULE_PATH}/internal/buildinfo.Commit=${COMMIT}"

# Prefer pinned release toolchain when installed; otherwise use active go.
if command -v go1.26.5 >/dev/null 2>&1; then
  GO_BIN="go1.26.5"
elif [[ -n "${GOTOOLCHAIN:-}" ]]; then
  GO_BIN="go"
else
  # Documented pin: release operators should set GOTOOLCHAIN=go1.26.5
  export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.5}"
  GO_BIN="go"
fi

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

echo "Building apifox-api ${VERSION} (commit ${COMMIT}) with ${GO_BIN} (GOTOOLCHAIN=${GOTOOLCHAIN:-default})"
"${GO_BIN}" version

for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  binary_name="apifox-api"
  if [[ "${goos}" == "windows" ]]; then
    binary_name="apifox-api.exe"
  fi
  artifact_dir="${OUT_DIR}/${goos}-${goarch}"
  mkdir -p "${artifact_dir}"
  artifact_path="${artifact_dir}/${binary_name}"
  echo "  -> ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${artifact_path}" .
done

# Checksums (SHA-256) for every artifact
checksum_file="${OUT_DIR}/checksums.txt"
: > "${checksum_file}"
(
  cd "${OUT_DIR}"
  find . -type f \( -name 'apifox-api' -o -name 'apifox-api.exe' \) | sort | while read -r relative_path; do
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "${relative_path}" >> checksums.txt
    else
      shasum -a 256 "${relative_path}" >> checksums.txt
    fi
  done
)

# Minimal SPDX-ish SBOM + provenance for the release tree (no secrets)
sbom_file="${OUT_DIR}/sbom.spdx.json"
provenance_file="${OUT_DIR}/provenance.json"
go_version_output="$("${GO_BIN}" version | tr -d '\r')"
module_path="$(go list -m -f '{{.Path}}' 2>/dev/null || echo github.com/akirousnow/apifox-api-go)"
cobra_version="$(go list -m -f '{{.Version}}' github.com/spf13/cobra 2>/dev/null || echo unknown)"

cat > "${sbom_file}" << SBOM
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "apifox-api-${VERSION}",
  "documentNamespace": "https://github.com/akirousnow/apifox-api-go@${VERSION}+${COMMIT}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: scripts/release.sh", "Organization: apifox-api"]
  },
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-apifox-api",
      "name": "apifox-api",
      "versionInfo": "${VERSION}",
      "downloadLocation": "NOASSERTION",
      "filesAnalyzed": false,
      "supplier": "Organization: apifox-api",
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:golang/${module_path}@${VERSION}"
        }
      ]
    },
    {
      "SPDXID": "SPDXRef-Package-cobra",
      "name": "github.com/spf13/cobra",
      "versionInfo": "${cobra_version}",
      "downloadLocation": "NOASSERTION",
      "filesAnalyzed": false
    }
  ]
}
SBOM

cat > "${provenance_file}" << PROV
{
  "predicateType": "https://slsa.dev/provenance/v0.2",
  "subject": [
    {"name": "apifox-api", "version": "${VERSION}", "commit": "${COMMIT}"}
  ],
  "predicate": {
    "builder": {"id": "scripts/release.sh"},
    "buildType": "https://go.dev/ref/mod#go-build",
    "invocation": {
      "parameters": {
        "targets": ["linux/amd64","linux/arm64","darwin/amd64","darwin/arm64","windows/amd64","windows/arm64"],
        "trimpath": true,
        "cgo": false,
        "ldflags": ["-s","-w","Version","Commit"]
      }
    },
    "materials": [
      {"uri": "git+local", "digest": {"commit": "${COMMIT}"}}
    ],
    "metadata": {
      "buildStartedOn": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
      "goVersion": "${go_version_output}",
      "toolchainPin": "go1.26.5",
      "module": "${module_path}"
    }
  }
}
PROV

# Linux smoke (host linux only)
if [[ "$(uname -s)" == "Linux" ]]; then
  smoke_bin="${OUT_DIR}/linux-amd64/apifox-api"
  if [[ -x "${smoke_bin}" ]]; then
    echo "Smoke: ${smoke_bin} --version"
    "${smoke_bin}" --version
    echo "Smoke: ${smoke_bin} version"
    "${smoke_bin}" version
  fi
fi

echo "Release artifacts written to ${OUT_DIR}"
ls -la "${OUT_DIR}"
echo "checksums:"
cat "${checksum_file}"
