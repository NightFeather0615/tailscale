// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

//go:build cgo || !darwin

package systray

import (
	"net/netip"
	"slices"
	"testing"

	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

func TestProfileTitleMultiline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		login     string
		tailnet   string
		multiline bool
		want      string
	}{
		{"no_tailnet", "alice@example.com", "", true, "alice@example.com"},
		{"dup_exact", "example.com", "example.com", true, "example.com"},
		{"dup_casefold", "Example.com", "example.com", false, "Example.com"},
		{"distinct_multiline", "alice@example.com", "example.com", true, "alice@example.com\nexample.com"},
		{"distinct_singleline", "alice@example.com", "example.com", false, "alice@example.com (example.com)"},
		{"empty", "", "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatProfileTitle(tt.login, tt.tailnet, tt.multiline); got != tt.want {
				t.Errorf("profileTitleMultiline; got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecommendedIsActive(t *testing.T) {
	t.Parallel()

	const (
		activeID = tailcfg.StableNodeID("active")
		suggID   = tailcfg.StableNodeID("suggestion")
	)
	usNYC := &tailcfg.Location{CountryCode: "US", City: "New York"}
	usCHI := &tailcfg.Location{CountryCode: "US", City: "Chicago"}
	seSTO := &tailcfg.Location{CountryCode: "SE", City: "Stockholm"}

	statusWith := func(activePeer *ipnstate.PeerStatus) *ipnstate.Status {
		s := &ipnstate.Status{
			ExitNodeStatus: &ipnstate.ExitNodeStatus{ID: activeID},
		}
		if activePeer != nil {
			s.Peer = map[key.NodePublic]*ipnstate.PeerStatus{{}: activePeer}
		}
		return s
	}

	tests := []struct {
		name        string
		status      *ipnstate.Status
		suggID      tailcfg.StableNodeID
		suggCountry string
		suggCity    string
		isActive    bool
	}{
		{
			name:   "nil_status",
			status: nil,
			suggID: suggID,
		},
		{
			name:   "no_exit_node",
			status: &ipnstate.Status{},
			suggID: suggID,
		},
		{
			name:   "exit_node_id_is_zero",
			status: &ipnstate.Status{ExitNodeStatus: &ipnstate.ExitNodeStatus{}},
			suggID: suggID,
		},
		{
			name:        "exact_id_match_short-circuits",
			status:      statusWith(&ipnstate.PeerStatus{ID: activeID, Location: usCHI}),
			suggID:      activeID,
			suggCountry: "US",
			suggCity:    "New York",
			isActive:    true,
		},
		{
			name:        "id_mismatch_but_same_city",
			status:      statusWith(&ipnstate.PeerStatus{ID: activeID, Location: usNYC}),
			suggID:      suggID,
			suggCountry: "US",
			suggCity:    "New York",
			isActive:    true,
		},
		{
			name:        "different_city",
			status:      statusWith(&ipnstate.PeerStatus{ID: activeID, Location: usCHI}),
			suggID:      suggID,
			suggCountry: "US",
			suggCity:    "New York",
		},
		{
			name:        "different_country",
			status:      statusWith(&ipnstate.PeerStatus{ID: activeID, Location: seSTO}),
			suggID:      suggID,
			suggCountry: "US",
			suggCity:    "New York",
		},
		{
			name:   "id_mismatch_suggestion_has_no_location",
			status: statusWith(&ipnstate.PeerStatus{ID: activeID, Location: usNYC}),
			suggID: suggID,
		},
		{
			name:        "id_mismatch_active_peer_has_no_location",
			status:      statusWith(&ipnstate.PeerStatus{ID: activeID}),
			suggID:      suggID,
			suggCountry: "US",
			suggCity:    "New York",
		},
		{
			name:        "active_peer_not_in_status",
			status:      statusWith(nil),
			suggID:      suggID,
			suggCountry: "US",
			suggCity:    "New York",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isExitNodeActive := recommendedIsActive(tt.status, tt.suggID, tt.suggCountry, tt.suggCity)
			if isExitNodeActive != tt.isActive {
				t.Errorf("recommendedIsActive; got %v, want %v", isExitNodeActive, tt.isActive)
			}
		})
	}
}

func TestNetworkDevices(t *testing.T) {
	t.Parallel()

	const me, other = tailcfg.UserID(1), tailcfg.UserID(2)
	ip := []netip.Addr{netip.MustParseAddr("100.64.0.1")}
	peer := func(dnsName, hostName string, user tailcfg.UserID) *ipnstate.PeerStatus {
		return &ipnstate.PeerStatus{DNSName: dnsName, HostName: hostName, UserID: user, TailscaleIPs: ip}
	}
	mullvad := peer("us-nyc-wg-301.mullvad.ts.net.", "us-nyc-wg-301", other)
	mullvad.Location = &tailcfg.Location{Country: "USA"}
	noIP := peer("noip.example.ts.net.", "noip", me)
	noIP.TailscaleIPs = nil

	status := &ipnstate.Status{
		Self: &ipnstate.PeerStatus{UserID: me},
		Peer: map[key.NodePublic]*ipnstate.PeerStatus{
			key.NewNode().Public(): peer("phone.example.ts.net.", "Pixel", me),
			key.NewNode().Public(): peer("Desktop.example.ts.net.", "desktop", me),
			key.NewNode().Public(): peer("", "no-dns-name", me),
			key.NewNode().Public(): peer("server.example.ts.net.", "server", other),
			key.NewNode().Public(): mullvad,
			key.NewNode().Public(): noIP,
		},
	}

	names := func(peers []*ipnstate.PeerStatus) (names []string) {
		for _, ps := range peers {
			names = append(names, deviceName(ps))
		}
		return names
	}
	mine, others := networkDevices(status)
	if got, want := names(mine), []string{"Desktop", "no-dns-name", "phone"}; !slices.Equal(got, want) {
		t.Errorf("my devices = %q; want %q", got, want)
	}
	if got, want := names(others), []string{"server"}; !slices.Equal(got, want) {
		t.Errorf("other devices = %q; want %q", got, want)
	}
}
