#!/bin/sh
set -e

CMD="$1"
AIRCMD_FILE="scripts/air-cmd.sh"
APP_NAME=${APP_NAME:-solana-validator-failover}

if [ "${CMD}" = "hot-reload" ]; then
    shift
    cat > "${AIRCMD_FILE}" << EOF
#!/bin/sh
bin/${APP_NAME}-dev-linux-amd64 $@
EOF
    chmod +x "${AIRCMD_FILE}"
fi


make "${CMD}"
