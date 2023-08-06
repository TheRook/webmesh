/*
Copyright 2023 Avi Zimmerman <avi.zimmerman@gmail.com>

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

// Package campfire implements the "camp fire" protocol.
package campfire

import (
	"fmt"
	"strings"

	"github.com/pion/datachannel"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"

	"github.com/webmeshproj/webmesh/pkg/context"
)

// WebRTC is the WebRTC API for Camp Fire connections.
var WebRTC *webrtc.API

func init() {
	s := webrtc.SettingEngine{}
	s.DetachDataChannels()
	WebRTC = webrtc.NewAPI(webrtc.WithSettingEngine(s))
}

// CampFire is a connection to one or more peers sharing the same pre-shared
// key.
type CampFire struct {
	*webrtc.PeerConnection
	datachannel.ReadWriteCloser

	errc   chan error
	readyc chan struct{}
	closec chan struct{}
}

// Options are options for creating or joining a new camp fire.
type Options struct {
	// PSK is the pre-shared key.
	PSK []byte
	// TURNServers is an optional list of turn servers to use.
	TURNServers []string
}

// New creates a new camp fire.
func New(ctx context.Context, opts Options) (*CampFire, error) {
	log := context.LoggerFrom(ctx).With("protocol", "campfire")
	loc, err := FindCampFire(opts.PSK, opts.TURNServers)
	if err != nil {
		return nil, fmt.Errorf("find camp fire: %w", err)
	}
	if !strings.HasPrefix(loc.TURNServer, "stun:") {
		loc.TURNServer = "stun:" + loc.TURNServer
	}
	log.Debug("found camp fire", "secret", loc.Secret, "turn", loc.TURNServer)
	conn, err := WebRTC.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{loc.TURNServer}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}
	cf := &CampFire{
		PeerConnection: conn,
		errc:           make(chan error, 1),
		closec:         make(chan struct{}),
	}
	cf.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Debug("ICE connection state changed", "state", state)
		if state == webrtc.ICEConnectionStateDisconnected {
			log.Debug("closing connection to camp fire")
			cf.PeerConnection.Close()
		}
	})
	cf.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		log.Debug("received ICE candidate", "candidate", c.String())
	})
	dc, err := cf.CreateDataChannel(loc.Secret, nil)
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("create data channel: %w", err)
	}
	dc.OnOpen(func() {
		log.Debug("data channel opened")
		rw, err := dc.Detach()
		if err != nil {
			log.Error("error detaching data channel", "error", err.Error())
			cf.errc <- err
			return
		}
		cf.ReadWriteCloser = rw
		close(cf.errc)
		close(cf.readyc)
	})
	offer, err := cf.CreateOffer(nil)
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("create offer: %w", err)
	}
	err = cf.SetLocalDescription(offer)
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("set local description: %w", err)
	}

	// Everything below broken
	localDesc := cf.LocalDescription()
	raw, err := localDesc.Unmarshal()
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("unmarshal local description: %w", err)
	}
	var fingerprint string
	for _, a := range raw.Attributes {
		if a.Key == "fingerprint" {
			fingerprint = a.Value
			break
		}
	}
	sd := &sdp.SessionDescription{}
	sd.Origin.Username = "-"
	sd.Origin.SessionID = 0
	sd.Origin.SessionVersion = 0
	sd.Origin.NetworkType = "IN"
	sd.Origin.AddressType = "IP4"
	sd.Origin.UnicastAddress = "0.0.0.0"
	sd.SessionName = sdp.SessionName(loc.Secret)
	sd.TimeDescriptions = append(sd.TimeDescriptions, sdp.TimeDescription{Timing: sdp.Timing{0, 0}})
	sd.ConnectionInformation = &sdp.ConnectionInformation{
		NetworkType: "IN",
		AddressType: "IP4",
		Address: &sdp.Address{
			Address: "0.0.0.0",
		},
	}
	sd = sd.WithFingerprint("sha-256", strings.TrimPrefix(fingerprint, "sha-256 "))
	sd.Attributes = append(sd.Attributes, sdp.Attribute{
		Key:   "ice-ufrag",
		Value: "-",
	})
	sd.Attributes = append(sd.Attributes, sdp.Attribute{
		Key:   "ice-pwd",
		Value: "-",
	})
	out, err := sd.Marshal()
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("marshal session description: %w", err)
	}
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  string(out),
	}
	err = cf.SetRemoteDescription(answer)
	if err != nil {
		defer cf.PeerConnection.Close()
		return nil, fmt.Errorf("set remote description: %w", err)
	}
	return cf, nil
}

func (cf *CampFire) Close() error {
	cf.PeerConnection.Close()
	return nil
}

func (cf *CampFire) Errors() <-chan error {
	return cf.errc
}

func (cf *CampFire) Ready() <-chan struct{} {
	return cf.readyc
}
