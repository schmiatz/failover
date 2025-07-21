package identities

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// Identities holds the information for the identities
type Identities struct {
	Active  *Identity
	Passive *Identity
}

// NewFromConfig creates a new identities from a config
func NewFromConfig(cfg *Config) (identities *Identities, err error) {
	logger := log.With().Str("component", "identities").Logger()
	identities = &Identities{}

	// load active identity
	logger.Debug().
		Str("file", cfg.Active).
		Msg("loading active identity")

	identities.Active, err = NewIdentityFromFile(cfg.Active)
	if err != nil {
		return nil, err
	}

	// load passive identity
	logger.Debug().
		Str("file", cfg.Passive).
		Msg("loading passive identity")

	identities.Passive, err = NewIdentityFromFile(cfg.Passive)
	if err != nil {
		return nil, err
	}

	// public keys must be different
	if identities.Active.Key.PublicKey() == identities.Passive.Key.PublicKey() {
		return nil, fmt.Errorf("active and passive identities must be different")
	}

	return
}
