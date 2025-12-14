package destination

import (
	"context"
	"fmt"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/vip"
)

type Entry struct {
	VIP       net.IP        `json:"vip"`
	VPort     uint16        `json:"vport"`
	Transport TransportType `json:"transport"`
	Address   TransportAddr `json:"address"`
}

func (e *Entry) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"vip":"%s","vport":%d,"transport":"%s","address":"%s"}`, e.VIP, e.VPort, e.Transport.String(), e.Address.String())), nil
}

type Entries struct {
	myVIP         net.IP
	featureRDMA   bool
	clientEntries map[uint64]map[uint64]map[uint16][][]*Entry // clientEntries[upper VIP][lower VIP][Vport][Transport]
	serverEntries map[uint16][]*Entry                         // serverEntries[Vport]
}

func newEntries(myVIP net.IP, featureRDMA bool) *Entries {
	return &Entries{
		myVIP:         myVIP,
		featureRDMA:   featureRDMA,
		clientEntries: make(map[uint64]map[uint64]map[uint16][][]*Entry),
		serverEntries: make(map[uint16][]*Entry),
	}
}

func (d *Entries) upsert(ctx context.Context, ip net.IP, port uint16, transport TransportType, address TransportAddr) {
	logger := log.FromContext(ctx).With("func", "destination.upsert", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)), "port", port, "transport", transport.String(), "address", address.String())

	if !d.featureRDMA && transport == TransportRDMA {
		return
	}
	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if _, ok := d.clientEntries[upper]; !ok {
		d.clientEntries[upper] = make(map[uint64]map[uint16][][]*Entry)
	}
	if _, ok := d.clientEntries[upper][lower]; !ok {
		d.clientEntries[upper][lower] = make(map[uint16][][]*Entry)
	}
	if _, ok := d.clientEntries[upper][lower][port]; !ok {
		d.clientEntries[upper][lower][port] = make([][]*Entry, NumTransportType)
	}
	d.clientEntries[upper][lower][port][transport] = append(d.clientEntries[upper][lower][port][transport], &Entry{
		VIP:       ip,
		VPort:     port,
		Transport: transport,
		Address:   address,
	})
	logger.DebugContext(ctx, "added to clientEntries")

	if ip.Equal(d.myVIP) {
		if _, ok := d.serverEntries[port]; !ok {
			d.serverEntries[port] = make([]*Entry, 0, 1)
		}
		d.serverEntries[port] = append(d.serverEntries[port], &Entry{
			VIP:       ip,
			VPort:     port,
			Transport: transport,
			Address:   address,
		})
		logger.DebugContext(ctx, "added to serverEntries")
	}
}

func (d *Entries) remove(ctx context.Context, ip net.IP, port uint16) {
	logger := log.FromContext(ctx).With("func", "destination.remove", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)), "port", port)

	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if _, ok := d.clientEntries[upper]; ok {
		if _, ok := d.clientEntries[upper][lower]; ok {
			if _, ok := d.clientEntries[upper][lower][port]; ok {
				for i := 0; i < NumTransportType; i++ {
					d.clientEntries[upper][lower][port][i] = nil
				}
				d.clientEntries[upper][lower][port] = nil
			}
		}
	}
	logger.DebugContext(ctx, "removed from clientEntries")

	if ip.Equal(d.myVIP) {
		if _, ok := d.serverEntries[port]; ok {
			d.serverEntries[port] = nil
		}
	}
	logger.DebugContext(ctx, "removed from serverEntries")
}

func (d *Entries) GetClient(ctx context.Context, ip net.IP, port uint16) [][]*Entry {
	logger := log.FromContext(ctx).With("func", "destination.GetClient", "ip", ip, "raw-ip", fmt.Sprintf("%+v", []byte(ip)), "port", port)

	upper, lower := vip.IP2Int(ip)
	logger.DebugContext(ctx, "parsed", "upper", upper, "lower", lower)
	if v1, ok := d.clientEntries[upper]; ok {
		if v2, ok := v1[lower]; ok {
			if v3, ok := v2[port]; ok {
				return v3
			}
		}
	}
	logger.DebugContext(ctx, "not found", "upper", upper, "lower", lower)
	return nil
}

func (d *Entries) GetServer(ctx context.Context, port uint16) []*Entry {
	logger := log.FromContext(ctx).With("func", "destination.GetServer", "port", port)

	if v, ok := d.serverEntries[port]; ok {
		return v
	}
	logger.DebugContext(ctx, "not found", "port", port)
	return nil
}
