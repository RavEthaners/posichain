package rpc

import (
	"context"
	"fmt"

	"github.com/PositionExchange/posichain/eth/rpc"
	nodeconfig "github.com/PositionExchange/posichain/internal/configs/node"
	"github.com/PositionExchange/posichain/internal/utils"
	"github.com/PositionExchange/posichain/p2p"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// PublicNetService offers network related RPC methods
type PublicNetService struct {
	net     p2p.Host
	chainID uint64
	version Version
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, chainID uint64, version Version) rpc.API {
	// manually set different namespace to preserve legacy behavior
	var namespace string
	switch version {
	case V1:
		namespace = netV1Namespace
	case V2:
		namespace = netV2Namespace
	case Eth:
		namespace = netNamespace
	default:
		utils.Logger().Error().Msgf("Unknown version %v, ignoring API.", version)
		return rpc.API{}
	}

	return rpc.API{
		Namespace: namespace,
		Version:   APIVersion,
		Service:   &PublicNetService{net, chainID, version},
		Public:    true,
	}
}

// PeerCount returns the number of connected peers
// Note that the return type is an interface to account for the different versions
func (s *PublicNetService) PeerCount(ctx context.Context) (interface{}, error) {
	timer := DoMetricRPCRequest(PeerCount)
	defer DoRPCRequestDuration(PeerCount, timer)
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(s.net.GetPeerCount()), nil
	case V2:
		return s.net.GetPeerCount(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// Version returns the network version, i.e. ChainID identifying which network we are using
func (s *PublicNetService) Version(ctx context.Context) interface{} {
	timer := DoMetricRPCRequest(NetVersion)
	defer DoRPCRequestDuration(NetVersion, timer)
	switch s.version {
	case Eth:
		return nodeconfig.GetDefaultConfig().GetNetworkType().ChainConfig().EthCompatibleChainID.String()
	default:
		return fmt.Sprintf("%d", s.chainID)
	}
}
