package accesscontrol

import (
	"context"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
)

type Manager struct {
	ServerEntries *Entries
	ClientEntries *Entries
}

func NewManager(defaultPolicy bool) *Manager {
	return &Manager{
		ServerEntries: newEntries(defaultPolicy),
		ClientEntries: newEntries(defaultPolicy),
	}
}

func (m *Manager) UpsertClient(ctx context.Context, srcIP net.IP, policy bool) {
	m.ClientEntries.upsert(ctx, srcIP, policy)
	log.FromContext(ctx).InfoContext(ctx, "client access control upserted", "srcIP", srcIP, "policy", policy)
}

func (m *Manager) RemoveClient(ctx context.Context, srcIP net.IP) {
	m.ClientEntries.remove(ctx, srcIP)
	log.FromContext(ctx).InfoContext(ctx, "client access control removed", "srcIP", srcIP)
}

func (m *Manager) UpsertServer(ctx context.Context, dstIP net.IP, policy bool) {
	m.ServerEntries.upsert(ctx, dstIP, policy)
	log.FromContext(ctx).InfoContext(ctx, "server access control upserted", "dstIP", dstIP, "policy", policy)
}

func (m *Manager) RemoveServer(ctx context.Context, dstIP net.IP) {
	m.ServerEntries.remove(ctx, dstIP)
	log.FromContext(ctx).InfoContext(ctx, "server access control removed", "dstIP", dstIP)
}
