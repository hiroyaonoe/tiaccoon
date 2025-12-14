package destination

import (
	"context"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
)

type Manager struct {
	Entries *Entries
}

func NewManager(myVIP net.IP, featureRDMA bool) *Manager {
	return &Manager{
		Entries: newEntries(myVIP, featureRDMA),
	}
}

func (m *Manager) Upsert(ctx context.Context, vip net.IP, vport uint16, transport TransportType, address TransportAddr) {
	m.Entries.upsert(ctx, vip, vport, transport, address)
	log.FromContext(ctx).InfoContext(ctx, "destination upserted", "vip", vip, "vport", vport, "transport", transport.String(), "address", address.String())
}

func (m *Manager) Remove(ctx context.Context, vip net.IP, vport uint16) {
	m.Entries.remove(ctx, vip, vport)
	log.FromContext(ctx).InfoContext(ctx, "destination removed", "vip", vip, "vport", vport)
}
