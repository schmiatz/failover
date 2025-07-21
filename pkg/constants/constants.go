package constants

import (
	// embed
	_ "embed"
)

var (
	// AppVersion ...
	//go:embed app.version
	AppVersion string
)

const (
	// AppName ...
	AppName = "solana-validator-failover"
	// AppEnvVarLogLevel ...
	AppEnvVarLogLevel = "SOLANA_FAILOVER_LOG_LEVEL"
)
