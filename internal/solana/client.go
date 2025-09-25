package solana

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog/log"
)

// RPCClientInterface defines the interface for RPC client operations - a solana rpc client interface
type RPCClientInterface interface {
	GetClusterNodes(ctx context.Context) ([]*rpc.GetClusterNodesResult, error)
	GetVoteAccounts(ctx context.Context, opts *rpc.GetVoteAccountsOpts) (*rpc.GetVoteAccountsResult, error)
	GetSlot(ctx context.Context, commitment rpc.CommitmentType) (uint64, error)
	GetLeaderSchedule(ctx context.Context) (rpc.GetLeaderScheduleResult, error)
	GetBlockTime(ctx context.Context, slot uint64) (*solanago.UnixTimeSeconds, error)
	GetHealth(ctx context.Context) (string, error)
	GetRecentPerformanceSamples(ctx context.Context, limit *uint64) ([]rpc.PerformanceSample, error)
}

// ClientInterface defines the interface for solana rpc operations - just simple wrappers around the rpc client
type ClientInterface interface {
	// NodeFromIP returns a Node from an IP address
	NodeFromIP(ip string) (*Node, error)
	// NodeFromPubkey returns a Node from a pubkey
	NodeFromPubkey(pubkey string) (*Node, error)
	// GetCreditRankedVoteAccountFromPubkey returns the credit rank-sorted current vote accounts rank is the difference
	// between current epoch credits and total credits (descending)
	GetCreditRankedVoteAccountFromPubkey(pubkey string) (*rpc.VoteAccountsResult, int, error)
	// GetCurrentSlot returns the current slot
	GetCurrentSlot() (slot uint64, err error)
	// GetCurrentSlotEndTime returns the end time of the current slot
	GetCurrentSlotEndTime() (time.Time, error)
	// GetTimeToNextLeaderSlotForPubkey returns the time to the next leader slot for the given pubkey
	GetTimeToNextLeaderSlotForPubkey(pubkey solanago.PublicKey) (isOnLeaderSchedule bool, timeToNextLeaderSlot time.Duration, err error)
	// GetLocalNodeHealth returns the health of the local node
	GetLocalNodeHealth() (string, error)
	// IsLocalNodeHealthy returns true if the local node is healthy
	IsLocalNodeHealthy() bool
}

// Client implements Interface using an RPC client
type Client struct {
	localRPCClient   RPCClientInterface
	networkRPCClient RPCClientInterface
	performanceCache struct {
		avgSlotTime  time.Duration
		lastUpdated  time.Time
		mutex        sync.RWMutex
	}
}

// NewClientParams is the parameters for creating a new client
type NewClientParams struct {
	LocalRPCURL   string
	NetworkRPCURL string
}

// NewRPCClient creates a new client for the given solana cluster
func NewRPCClient(params NewClientParams) ClientInterface {
	return &Client{
		localRPCClient:   rpc.New(params.LocalRPCURL),
		networkRPCClient: rpc.New(params.NetworkRPCURL),
	}
}

// GetLocalNodeHealth returns the health of the local node
func (c *Client) GetLocalNodeHealth() (string, error) {
	result, err := c.localRPCClient.GetHealth(context.Background())
	if err != nil {
		return err.Error(), fmt.Errorf("failed to get local node health: %w", err)
	}
	return string(result), nil
}

// IsLocalNodeHealthy returns true if the local node is healthy
func (c *Client) IsLocalNodeHealthy() bool {
	result, err := c.GetLocalNodeHealth()
	if err != nil {
		log.Debug().Err(err).Msg("failed to get local node health")
		return false
	}
	isHealthy := result == rpc.HealthOk
	if !isHealthy {
		log.Debug().Str("result", result).Msg("local node health")
	}
	return isHealthy
}

// NodeFromIP returns a Node from an IP address
func (c *Client) NodeFromIP(ip string) (*Node, error) {
	gossipNode, err := c.nodeFromIP(ip)
	if err != nil {
		return nil, err
	}
	return &Node{gossipNode: gossipNode}, nil
}

// NodeFromPubkey returns a Node from a pubkey
func (c *Client) NodeFromPubkey(pubkey string) (*Node, error) {
	gossipNode, err := c.gossipNodeFromPubkey(pubkey)
	if err != nil {
		return nil, err
	}
	return &Node{gossipNode: gossipNode}, nil
}

func (c *Client) nodeFromIP(ip string) (node *rpc.GetClusterNodesResult, err error) {
	nodes, err := c.networkRPCClient.GetClusterNodes(context.Background())
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.Gossip != nil {
			gossipIP := strings.Split(*node.Gossip, ":")[0]
			if gossipIP == ip {
				return node, nil
			}
		}
	}

	return nil, fmt.Errorf("gossip node not found for ip: %s", ip)
}

func (c *Client) gossipNodeFromPubkey(pubkey string) (node *rpc.GetClusterNodesResult, err error) {
	nodes, err := c.networkRPCClient.GetClusterNodes(context.Background())
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.Pubkey.String() == pubkey {
			return node, nil
		}
	}

	return nil, fmt.Errorf("gossip node not found for pubkey: %s", pubkey)
}

// GetCreditRankedVoteAccountFromPubkey returns the credit rank-sorted current vote accounts rank is the difference
// between current epoch credits and total credits (descending)
func (c *Client) GetCreditRankedVoteAccountFromPubkey(pubkey string) (voteAccount *rpc.VoteAccountsResult, creditRank int, err error) {
	// fetch all vote accounts
	voteAccounts, err := c.networkRPCClient.GetVoteAccounts(
		context.Background(),
		&rpc.GetVoteAccountsOpts{
			Commitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get vote account from pubkey %s: %w", pubkey, err)
	}

	// select current (non-delinquent) vote accounts
	currentVoteAccounts := voteAccounts.Current

	// sort validators by the difference between current epoch credits and total credits (descending)
	sort.SliceStable(currentVoteAccounts, func(i, j int) bool {
		// calculate the difference between current epoch credits and total credits
		var iDiff, jDiff int64
		if len(currentVoteAccounts[i].EpochCredits) > 0 {
			lastIndex := len(currentVoteAccounts[i].EpochCredits) - 1
			currentCredits := currentVoteAccounts[i].EpochCredits[lastIndex][1]
			totalCredits := currentVoteAccounts[i].EpochCredits[lastIndex][2]
			iDiff = currentCredits - totalCredits
		}
		if len(currentVoteAccounts[j].EpochCredits) > 0 {
			lastIndex := len(currentVoteAccounts[j].EpochCredits) - 1
			currentCredits := currentVoteAccounts[j].EpochCredits[lastIndex][1]
			totalCredits := currentVoteAccounts[j].EpochCredits[lastIndex][2]
			jDiff = currentCredits - totalCredits
		}
		return iDiff > jDiff
	})

	for i, account := range currentVoteAccounts {
		if account.NodePubkey.String() == pubkey {
			creditRank = i + 1 // rank is 1-indexed
			return &account, creditRank, nil
		}
	}

	return nil, 0, fmt.Errorf("vote account not found for pubkey: %s", pubkey)
}

// GetCurrentSlot returns the current slot
func (c *Client) GetCurrentSlot() (slot uint64, err error) {
	slot, err = c.networkRPCClient.GetSlot(context.Background(), rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("failed to get slot: %w", err)
	}
	return slot, nil
}

// GetCurrentSlotEndTime returns the end time of the current slot
func (c *Client) GetCurrentSlotEndTime() (time.Time, error) {
	slot, err := c.GetCurrentSlot()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get current slot: %w", err)
	}

	expectedCurrentSlotEndTime, err := c.networkRPCClient.GetBlockTime(context.Background(), slot)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get block time for current slot: %w", err)
	}

	// if no estimate availabe, assume 400ms from now
	if expectedCurrentSlotEndTime == nil {
		return time.Now().UTC().Add(400 * time.Millisecond), nil
	}

	// return the time in utc
	return time.Unix(int64(*expectedCurrentSlotEndTime), 0).UTC(), nil
}

// GetTimeToNextLeaderSlotForPubkey returns the time to the next leader slot for the given pubkey
func (c *Client) GetTimeToNextLeaderSlotForPubkey(pubkey solanago.PublicKey) (isOnLeaderSchedule bool, timeToNextLeaderSlot time.Duration, err error) {
	// get the current slot
	currentSlot, err := c.GetCurrentSlot()
	if err != nil {
		return false, time.Duration(0), fmt.Errorf("failed to get current slot: %w", err)
	}

	// get the leader schedule
	leaderSchedule, err := c.networkRPCClient.GetLeaderSchedule(context.Background())
	if err != nil {
		return false, time.Duration(0), fmt.Errorf("failed to get leader schedule: %w", err)
	}

	// get upcoming slots fo the pubkey
	slots, ok := leaderSchedule[pubkey]

	// pubkey not in leader schedule
	if !ok {
		return false, time.Duration(0), nil
	}

	var nextLeaderSlot uint64

	for _, s := range slots {
		if s > currentSlot {
			nextLeaderSlot = s
			break
		}
	}

	// didn't find future slots for the pubkey
	if nextLeaderSlot == 0 {
		// return indefinite time
		return false, time.Duration(0), nil
	}

	// Calculate slots until leader slot
	slotsUntilLeader := nextLeaderSlot - currentSlot
	
	// Get average slot time from recent performance
	avgSlotTime, err := c.getAverageSlotTime()
	if err != nil {
		return false, time.Duration(0), fmt.Errorf("failed to get average slot time: %w", err)
	}
	
	// Calculate time to next leader slot based on slots and average slot time
	timeToNextLeaderSlot = time.Duration(slotsUntilLeader) * avgSlotTime

	return true, timeToNextLeaderSlot, nil
}

// getAverageSlotTime returns the average slot time based on recent performance samples
// Uses caching to avoid excessive RPC calls and includes retry logic with fallback
func (c *Client) getAverageSlotTime() (time.Duration, error) {
	// Check cache first (valid for 30 seconds)
	c.performanceCache.mutex.RLock()
	if time.Since(c.performanceCache.lastUpdated) < 30*time.Second {
		avgSlotTime := c.performanceCache.avgSlotTime
		c.performanceCache.mutex.RUnlock()
		return avgSlotTime, nil
	}
	c.performanceCache.mutex.RUnlock()

	// Cache expired, fetch new data
	c.performanceCache.mutex.Lock()
	defer c.performanceCache.mutex.Unlock()

	// Double-check in case another goroutine updated it
	if time.Since(c.performanceCache.lastUpdated) < 30*time.Second {
		return c.performanceCache.avgSlotTime, nil
	}

	// Retry logic for performance samples
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		limit := uint64(10)
		samples, err := c.networkRPCClient.GetRecentPerformanceSamples(context.Background(), &limit)
		if err == nil && len(samples) > 0 && samples[0].NumSlots > 0 {
			sample := samples[0]
			avgSlotTimeMs := float64(sample.SamplePeriodSecs) / float64(sample.NumSlots) * 1000
			avgSlotTime := time.Duration(avgSlotTimeMs) * time.Millisecond
			
			// Update cache
			c.performanceCache.avgSlotTime = avgSlotTime
			c.performanceCache.lastUpdated = time.Now()
			
			log.Debug().
				Float64("avg_slot_time_ms", avgSlotTimeMs).
				Int("num_slots", sample.NumSlots).
				Int("sample_period_secs", sample.SamplePeriodSecs).
				Msg("updated performance cache with recent samples")
			
			return avgSlotTime, nil
		}
		
		if i < maxRetries-1 {
			log.Debug().Err(err).Msgf("failed to get performance samples, retrying in 1s (attempt %d/%d)", i+1, maxRetries)
			time.Sleep(1 * time.Second) // Wait 1 second before retry
		}
	}

	// All retries failed, use fallback
	fallbackTime := 400 * time.Millisecond
	c.performanceCache.avgSlotTime = fallbackTime
	c.performanceCache.lastUpdated = time.Now()
	
	log.Warn().
		Dur("fallback_time", fallbackTime).
		Msg("failed to get performance samples after retries, using fallback slot time")
	
	return fallbackTime, nil
}
