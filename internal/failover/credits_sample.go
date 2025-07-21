package failover

import "time"

// CreditsSample represents a sample of the vote credits for a given identity
type CreditsSample struct {
	VoteAccountPubkey string
	VoteRank          int
	Credits           int
	Timestamp         time.Time
}

// CreditSamples is a map of identity pubkeys to their vote credits samples
type CreditSamples map[string][]CreditsSample
