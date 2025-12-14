package accesscontrol

import (
	"context"
	"fmt"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/vip"
)

type Entries struct {
	defaultPolicy bool
	entries       map[uint64]map[uint64]bool // entries[upper VIP][lower VIP]
}

func newEntries(defaultPolicy bool) *Entries {
	return &Entries{
		defaultPolicy: defaultPolicy,
		entries:       make(map[uint64]map[uint64]bool),
	}
}

func (a *Entries) upsert(ctx context.Context, ip net.IP, policy bool) {
	logger := log.FromContext(ctx).With("func", "accesscontrol.upsert", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)), "policy", policy)
	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if _, ok := a.entries[upper]; !ok {
		a.entries[upper] = make(map[uint64]bool)
	}
	a.entries[upper][lower] = policy
}

func (a *Entries) remove(ctx context.Context, ip net.IP) {
	logger := log.FromContext(ctx).With("func", "accesscontrol.remove", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)))
	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if _, ok := a.entries[upper]; ok {
		delete(a.entries[upper], lower)
	}
}

func (a *Entries) Apply(ctx context.Context, ip net.IP) bool {
	logger := log.FromContext(ctx).With("func", "accesscontrol.Apply", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)))
	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if v, ok := a.entries[upper]; ok {
		if policy, ok := v[lower]; ok {
			return policy
		}
	}
	return a.defaultPolicy
}
