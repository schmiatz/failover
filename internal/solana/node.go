package solana

import (
	"strings"

	"github.com/gagliardetto/solana-go/rpc"
)

// Node represents a gossip node
type Node struct {
	gossipNode *rpc.GetClusterNodesResult
}

// IP returns the IP address of the gossip node
func (n *Node) IP() string {
	return strings.Split(*n.gossipNode.Gossip, ":")[0]
}

// Pubkey returns the pubkey of the gossip node
func (n *Node) Pubkey() string {
	return n.gossipNode.Pubkey.String()
}

// Version returns the version of the gossip node
func (n *Node) Version() string {
	return *n.gossipNode.Version
}

// Refresh refreshes the gossip node using the provided gossip client
func (n *Node) Refresh(gossipClient ClientInterface) error {
	refreshedNode, err := gossipClient.NodeFromIP(n.IP())
	if err != nil {
		return err
	}
	n.gossipNode = refreshedNode.gossipNode
	return nil
}
