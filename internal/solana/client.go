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
	GetEpochInfo(ctx context.Context, commitment rpc.CommitmentType) (*rpc.GetEpochInfoResult, error)
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

	// get epoch info to calculate first slot of current epoch
	epochInfo, err := c.networkRPCClient.GetEpochInfo(context.Background(), rpc.CommitmentProcessed)
	if err != nil {
		return false, time.Duration(0), fmt.Errorf("failed to get epoch info: %w", err)
	}

	// calculate first slot of current epoch
	firstSlotOfEpoch := epochInfo.AbsoluteSlot - epochInfo.SlotIndex

	log.Debug().
		Uint64("current_slot", currentSlot).
		Uint64("absolute_slot", epochInfo.AbsoluteSlot).
		Uint64("slot_index", epochInfo.SlotIndex).
		Uint64("first_slot_of_epoch", firstSlotOfEpoch).
		Uint64("epoch", epochInfo.Epoch).
		Msg("epoch info for leader slot calculation")

	// get the leader schedule (returns relative slot indices within the epoch)
	leaderSchedule, err := c.networkRPCClient.GetLeaderSchedule(context.Background())
	if err != nil {
		return false, time.Duration(0), fmt.Errorf("failed to get leader schedule: %w", err)
	}

	// get upcoming slots for the pubkey (these are relative slot indices)
	relativeSlots, ok := leaderSchedule[pubkey]

	// pubkey not in leader schedule
	if !ok {
		log.Debug().
			Str("validator_pubkey", pubkey.String()).
			Int("total_validators_in_schedule", len(leaderSchedule)).
			Msg("validator not found in leader schedule")
		return false, time.Duration(0), nil
	}

	var nextLeaderSlot uint64

	log.Debug().
		Str("validator_pubkey", pubkey.String()).
		Uint64("current_slot", currentSlot).
		Uint64("first_slot_of_epoch", firstSlotOfEpoch).
		Int("total_relative_slots", len(relativeSlots)).
		Msg("checking relative slots for future leader slots")

	// Convert relative slots to absolute slots and find the next future slot
	for _, relativeSlot := range relativeSlots {
		absoluteSlot := firstSlotOfEpoch + relativeSlot
		
		log.Debug().
			Uint64("relative_slot", relativeSlot).
			Uint64("absolute_slot", absoluteSlot).
			Uint64("current_slot", currentSlot).
			Bool("is_future", absoluteSlot > currentSlot).
			Msg("checking converted slot")
		
		if absoluteSlot > currentSlot {
			nextLeaderSlot = absoluteSlot
			log.Debug().
				Uint64("next_leader_slot", nextLeaderSlot).
				Msg("found next future leader slot")
			break
		}
	}

	// didn't find future slots for the pubkey
	if nextLeaderSlot == 0 {
		log.Debug().
			Str("validator_pubkey", pubkey.String()).
			Uint64("current_slot", currentSlot).
			Uint64("first_slot_of_epoch", firstSlotOfEpoch).
			Int("total_relative_slots", len(relativeSlots)).
			Msg("validator found in leader schedule but has no future slots in current epoch")
		
		// Log some sample relative slots for debugging
		if len(relativeSlots) > 0 {
			sampleSlots := relativeSlots
			if len(relativeSlots) > 5 {
				sampleSlots = relativeSlots[:5]
			}
			log.Debug().
				Uints64("sample_relative_slots", sampleSlots).
				Msg("sample relative slots from leader schedule")
		}
		
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

	log.Debug().
		Uint64("next_leader_slot", nextLeaderSlot).
		Uint64("current_slot", currentSlot).
		Uint64("slots_until_leader", slotsUntilLeader).
		Dur("avg_slot_time", avgSlotTime).
		Dur("time_to_next_leader_slot", timeToNextLeaderSlot).
		Msg("calculated time to next leader slot")

	return true, timeToNextLeaderSlot, nil
}

// getAverageSlotTime returns the average slot time
// Uses a fixed 400ms slot time as a reasonable approximation for Solana
// TODO: Could be enhanced to use getRecentPerformanceSamples for dynamic calculation
func (c *Client) getAverageSlotTime() (time.Duration, error) {
	// Check cache first (valid for 30 seconds)
	c.performanceCache.mutex.RLock()
	if time.Since(c.performanceCache.lastUpdated) < 30*time.Second {
		avgSlotTime := c.performanceCache.avgSlotTime
		c.performanceCache.mutex.RUnlock()
		return avgSlotTime, nil
	}
	c.performanceCache.mutex.RUnlock()

	// Cache expired, update with fixed slot time
	c.performanceCache.mutex.Lock()
	defer c.performanceCache.mutex.Unlock()

	// Double-check in case another goroutine updated it
	if time.Since(c.performanceCache.lastUpdated) < 30*time.Second {
		return c.performanceCache.avgSlotTime, nil
	}

	// Use fixed 400ms slot time (reasonable approximation for Solana)
	avgSlotTime := 400 * time.Millisecond
	c.performanceCache.avgSlotTime = avgSlotTime
	c.performanceCache.lastUpdated = time.Now()
	
	log.Debug().
		Dur("avg_slot_time", avgSlotTime).
		Msg("using fixed slot time for leader slot calculation")
	
	return avgSlotTime, nil
}
