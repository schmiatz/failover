#!/bin/sh
set -e
. "${0%/*}"/logger.sh

APP_NAME=${APP_NAME:-solana-validator-failover}
GITHUB_TOKEN=${GITHUB_TOKEN:-}
GITHUB_REPO=${GITHUB_REPO:-}
REPO_TAG=${REPO_TAG:-}
APP_VERSION=${APP_VERSION:-}

gh_create_release() {
    log_info "creating release notes"
    cat > release-notes.md <<EOF
## Release ${APP_VERSION}

### Installation
Download the appropriate binary for your platform and extract it:
\`\`\`bash
# Download and extract
wget https://github.com/sol-strategies/solana-validator-failover/releases/download/${REPO_TAG}/solana-validator-failover-${APP_VERSION}-<platform>.gz
gunzip solana-validator-failover-${APP_VERSION}-<platform>.gz
chmod +x solana-validator-failover-${APP_VERSION}-<platform>
\`\`\`

### Verification
Each binary includes a SHA256 checksum file for integrity verification.

To verify a binary:
\`\`\`bash
sha256sum -c solana-validator-failover-${APP_VERSION}-<platform>.sha256
\`\`\`
EOF

    log_info "creating GitHub release"
    gh release create ${REPO_TAG} \
        --repo ${GITHUB_REPO} \
        --title "Release ${REPO_TAG}" \
        --notes-file release-notes.md \
        --draft=false \
        --prerelease=false
}

gh_release_upload_assets() {
    log_info "uploading release assets"
    cd bin
    
    # Compress and upload binaries
    for file in *; do
        if [ -f "$file" ] && [ ! -f "${file}.gz" ] && [ ! -f "${file}.sha256" ]; then
            echo "Compressing $file..."
            gzip -9 "$file"
            echo "Uploading ${file}.gz..."
            gh release upload ${REPO_TAG} "${file}.gz" \
                --repo ${GITHUB_REPO} \
                --clobber
        fi
    done
    
    # Upload checksum files without compression
    for file in *.sha256; do
        if [ -f "$file" ]; then
            echo "Uploading $file..."
            gh release upload ${REPO_TAG} "$file" \
                --repo ${GITHUB_REPO} \
                --clobber
        fi
    done
}

main() {
    gh_create_release
    gh_release_upload_assets
}

main
