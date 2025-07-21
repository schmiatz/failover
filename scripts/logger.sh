#!/bin/sh

# Simple logger
_log() {
    local msgLevel=${1}
    shift || return 0
    local logLevel=${LOG_LEVEL:-INFO}
    # make sure log level is uppercase
    logLevel=$(echo "${logLevel}" | awk '{ print toupper($1) }')

    # space padding to match longest warning level - WARNING
    local paddedMsgLevel=" ${msgLevel}"
    case "${msgLevel}" in
    ERROR | DEBUG)
        paddedMsgLevel="   ${msgLevel}"
        ;;
    INFO)
        paddedMsgLevel="    ${msgLevel}"
        ;;
    esac

    # pretty log enntry
    local logEntry="[$(date -u +"%F %T %Z") ${paddedMsgLevel}]: ${@}"

    # Choose level to print depending on LOG_LEVEL env var
    case "${logLevel}" in
    ERROR)
        case ${msgLevel} in
        ERROR) echo -e "${logEntry}";;
        esac
        ;;
    WARNING)
        case ${msgLevel} in
        ERROR | WARNING) echo -e "${logEntry}" ;;
        esac
        ;;
    INFO)
        case ${msgLevel} in
        ERROR | WARNING | INFO) echo -e "${logEntry}" ;;
        esac
        ;;
    DEBUG)
        case ${msgLevel} in
        ERROR | WARNING | INFO | DEBUG) echo -e "${logEntry}" ;;
        esac
        ;;
    esac
    return 0
}

log_error() {
    _log "ERROR" "$@"
}

log_warning() {
    _log "WARNING" "$@"
}

log_info() {
    _log "INFO" "$@"
}

log_debug() {
    _log "DEBUG" "$@"
}
