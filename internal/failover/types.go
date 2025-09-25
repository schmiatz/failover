package failover

// MonitorConfig holds the configuration for a failover monitor
type MonitorConfig struct {
	CreditSamples CreditSamplesConfig `mapstructure:"credit_samples"`
}

// CreditSamplesConfig holds the configuration for a failover monitor credit samples
type CreditSamplesConfig struct {
	Count    int    `mapstructure:"count"`
	Interval string `mapstructure:"interval"`
}
