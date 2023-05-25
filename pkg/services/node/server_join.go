/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "gitlab.com/webmesh/api/v1"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.com/webmesh/node/pkg/meshdb/peers"
	"gitlab.com/webmesh/node/pkg/util"
)

func (s *Server) Join(ctx context.Context, req *v1.JoinRequest) (*v1.JoinResponse, error) {
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not leader")
	}

	if !s.ulaPrefix.IsValid() {
		var err error
		s.ulaPrefix, err = s.meshstate.GetULAPrefix(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get ULA prefix: %v", err)
		}
	}

	// Validate inputs
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node id required")
	}
	publicKey, err := wgtypes.ParseKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}
	var primaryEndpoint netip.Addr
	if req.GetPrimaryEndpoint() != "" {
		primaryEndpoint, err = netip.ParseAddr(req.GetPrimaryEndpoint())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid primary endpoint: %v", err)
		}
	}
	var endpoints []netip.Addr
	if len(req.GetEndpoints()) > 0 {
		for _, endpointStr := range req.GetEndpoints() {
			endpoint, err := netip.ParseAddr(endpointStr)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid endpoint: %v", err)
			}
			endpoints = append(endpoints, endpoint)
		}
	}

	log := s.log.With("id", req.GetId())

	// Check if the peer already exists
	var peer *peers.Node
	peer, err = s.peers.Get(ctx, req.GetId())
	if err != nil && err != peers.ErrNodeNotFound {
		// Database error
		return nil, status.Errorf(codes.Internal, "failed to get peer: %v", err)
	} else if err == nil {
		log.Info("peer already exists, checking for updates")
		// Peer already exists, update it
		if peer.PublicKey.String() != publicKey.String() {
			peer.PublicKey = publicKey
		}
		if peer.GRPCPort != int(req.GetGrpcPort()) {
			peer.GRPCPort = int(req.GetGrpcPort())
		}
		if peer.RaftPort != int(req.GetRaftPort()) {
			peer.RaftPort = int(req.GetRaftPort())
		}
		if peer.WireguardPort != int(req.GetWireguardPort()) {
			peer.WireguardPort = int(req.GetWireguardPort())
		}
		if primaryEndpoint.IsValid() && primaryEndpoint.String() != peer.PrimaryEndpoint.String() {
			peer.PrimaryEndpoint = primaryEndpoint
		}
		if !cmp.Equal(peer.Endpoints, endpoints) {
			peer.Endpoints = endpoints
		}
		peer, err = s.peers.Update(ctx, peer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update peer: %v", err)
		}
	} else {
		// New peer, create it
		log.Info("registering new peer")
		networkIPv6, err := util.Random64(s.ulaPrefix)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate IPv6 address: %v", err)
		}
		peer, err = s.peers.Create(ctx, &peers.CreateOptions{
			ID:              req.GetId(),
			PublicKey:       publicKey,
			PrimaryEndpoint: primaryEndpoint,
			Endpoints:       endpoints,
			NetworkIPv6:     networkIPv6,
			GRPCPort:        int(req.GetGrpcPort()),
			RaftPort:        int(req.GetRaftPort()),
			WireguardPort:   int(req.GetWireguardPort()),
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create peer: %v", err)
		}
	}

	// Start building the response
	resp := &v1.JoinResponse{
		NetworkIpv6: peer.NetworkIPv6.String(),
	}
	var lease netip.Prefix
	if req.GetAssignIpv4() {
		log.Info("assigning IPv4 address to peer")
		lease, err = s.ipam.Acquire(ctx, req.GetId())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to assign IPv4: %v", err)
		}
		log.Info("assigned IPv4 address to peer", slog.String("ipv4", lease.String()))
		resp.AddressIpv4 = lease.String()
	}
	// Fetch current wireguard peers for the new node
	peers, err := s.peers.ListPeers(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list peers: %v", err)
	}
	// Add peer to the raft cluster
	var raftAddress string
	if req.GetAssignIpv4() && !req.GetPreferRaftIpv6() {
		// Prefer IPv4 for raft
		raftAddress = net.JoinHostPort(lease.Addr().String(), strconv.Itoa(peer.RaftPort))
	} else {
		// Use IPv6
		// TODO: doesn't work when we are IPv4 only. Need to fix this.
		// Basically if a single node is IPv4 only, we need to use IPv4 for raft.
		// We may as well use IPv4 for everything in that case.
		raftAddress = net.JoinHostPort(peer.NetworkIPv6.Addr().String(), strconv.Itoa(peer.RaftPort))
	}
	if req.GetAsVoter() {
		log.Info("adding candidate to cluster", slog.String("raft_address", raftAddress))
		if err := s.store.AddVoter(ctx, req.GetId(), raftAddress); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to add candidate: %v", err)
		}
	} else {
		log.Info("adding non-voter to cluster", slog.String("raft_address", raftAddress))
		if err := s.store.AddNonVoter(ctx, req.GetId(), raftAddress); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to add non-voter: %v", err)
		}
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.store.RefreshWireguardPeers(ctx); err != nil {
			log.Warn("failed to refresh wireguard peers", slog.String("error", err.Error()))
		}
	}()
	resp.Peers = make([]*v1.WireguardPeer, len(peers))
	for i, p := range peers {
		peer := p
		resp.Peers[i] = &v1.WireguardPeer{
			Id:        peer.ID,
			PublicKey: peer.PublicKey.String(),
			// TODO: This still assumes fairly simple setups. We need to handle situations
			// where two nodes wish to be bridged over NAT64 or ICE. If a single node provides
			// NAT64 to the network, this becomes a lot easier.
			//
			// For now, when a peer behind a NAT receives an endpoint it can contact, it allows
			// all traffic from that endpoint. This is not ideal, but it works.
			PrimaryEndpoint: func() string {
				if peer.PrimaryEndpoint.IsValid() {
					return netip.AddrPortFrom(peer.PrimaryEndpoint, uint16(peer.WireguardPort)).String()
				}
				return ""
			}(),
			Endpoints: func() []string {
				if len(peer.Endpoints) > 0 {
					endpointStrs := make([]string, len(peer.Endpoints))
					for i, endpoint := range peer.Endpoints {
						endpointStrs[i] = netip.AddrPortFrom(endpoint, uint16(peer.WireguardPort)).String()
					}
					return endpointStrs
				}
				return nil
			}(),
			AddressIpv4: func() string {
				if peer.PrivateIPv4.IsValid() {
					return peer.PrivateIPv4.String()
				}
				return ""
			}(),
			AddressIpv6: func() string {
				if peer.NetworkIPv6.IsValid() {
					return peer.NetworkIPv6.String()
				}
				return ""
			}(),
		}
	}
	return resp, nil
}
