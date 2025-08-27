package identities

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/rs/zerolog/log"
	"github.com/sol-strategies/solana-validator-failover/internal/utils"
)

// Identity holds the information for an identity
type Identity struct {
	KeyFile string // path to the identity key file
	Key     solana.PrivateKey
}

// NewIdentityFromFile Identity from a key file
func NewIdentityFromFile(keyFile string) (identity *Identity, err error) {
	logger := log.With().Str("component", "identities").Logger()
	// resolve path
	keyFileAbsolutePath, err := utils.ResolvePath(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	identity = &Identity{
		KeyFile: keyFileAbsolutePath,
	}

	logger.Debug().
		Str("file", keyFileAbsolutePath).
		Msg("reading solana keygen file")

	identity.Key, err = solana.PrivateKeyFromSolanaKeygenFile(keyFileAbsolutePath)
	if err != nil {
		err = fmt.Errorf("failed to parse keygen file: %w", err)
		return
	}

	logger.Debug().
		Str("pubkey", identity.Key.PublicKey().String()).
		Str("file", keyFileAbsolutePath).
		Msg("parsed solana keygen file")

	return identity, nil
}

// Pubkey returns the public key of the identity - prefer its PascalCase counterpart PubKey
func (i *Identity) Pubkey() string {
	log.Warn().Msg("Pubkey is deprecated (but still works) in favour of PubKey - using it for you...")
	return i.PubKey()
}

// PubKey is the PascalCase counterpart of Pubkey - it's what we should have always used but let's be honest about why not:
// @coderigo messed up in the early README and claimed PubKey was supported when it was really Pubkey
func (i *Identity) PubKey() string {
	return i.Key.PublicKey().String()
}
