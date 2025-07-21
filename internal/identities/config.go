package identities

// Config holds the configuration for the identities this validator can assume
// depending on the role it is assigned
type Config struct {
	Active  string `mapstructure:"active"`
	Passive string `mapstructure:"passive"`
}
