package manage

import (
	"context"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/accesscontrol"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
)

type Manager struct {
	am            *accesscontrol.Manager
	dm            *destination.Manager
	defaultPolicy bool
	myVIP         net.IP
	featureRDMA   bool
}

func NewManager(defaultPolicy bool, myVIP net.IP, featureRDMA bool) *Manager {
	return &Manager{
		defaultPolicy: defaultPolicy,
		myVIP:         myVIP,
		featureRDMA:   featureRDMA,
	}
}

func (m *Manager) Close(ctx context.Context) {
	logger := log.FromContext(ctx).With("component", "manager")
	logger.DebugContext(ctx, "Closing manager")
}

func (m *Manager) Start(ctx context.Context) (sae, cae *accesscontrol.Entries, de *destination.Entries) {
	logger := log.FromContext(ctx).With("component", "manager")
	ctx = log.ContextWithLogger(ctx, logger)
	logger.DebugContext(ctx, "Starting manager")

	m.am = accesscontrol.NewManager(m.defaultPolicy)
	m.dm = destination.NewManager(m.myVIP, m.featureRDMA)

	go m.manage(ctx)

	return m.am.ServerEntries, m.am.ClientEntries, m.dm.Entries
}

func (m *Manager) manage(ctx context.Context) {
	// TODO: Implement the logic to manage the entries
	m.am.UpsertClient(ctx, net.IPv4(10, 0, 10, 50), true)

	// TODO: allow zero bind (dynamic port)
	m.dm.Upsert(ctx,
		net.IPv4(0, 0, 0, 0),
		0,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 0),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		0,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 0),
	)

	var yourVIP net.IP
	if m.myVIP.Equal(net.IPv4(10, 0, 10, 40)) {
		yourVIP = net.IPv4(10, 0, 10, 50)
	} else {
		yourVIP = net.IPv4(10, 0, 10, 40)
	}

	// netperf
	// m.netperfTCPLocal(ctx, yourVIP)
	// m.netperfTCPRemote(ctx, yourVIP)
	// m.netperfUNIX(ctx, yourVIP)
	// m.netperfRDMALocal(ctx, yourVIP)
	// m.netperfRDMARemote(ctx, yourVIP)

	m.nginxTCPLocal(ctx, yourVIP)

	// m.testDestination(ctx, yourVIP)
}

func (m *Manager) testDestination(ctx context.Context, yourVIP net.IP) {
	// ClientEntry
	m.dm.Upsert(ctx,
		net.IPv4(127, 0, 0, 1),
		6000,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{127, 0, 0, 1}, 18000),
	)
	m.dm.Upsert(ctx,
		net.IPv4(127, 0, 0, 1),
		8000,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{192, 168, 20, 3}, 18000),
	)
	m.dm.Upsert(ctx,
		net.IPv4(127, 0, 0, 1),
		9000,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/test.sock"),
	)
	m.dm.Upsert(ctx,
		net.IPv4(127, 0, 0, 1),
		7000,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{192, 168, 20, 30}, 17000),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		8000,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 18000),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		8000,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 28000),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		8000,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/test.sock"),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		8000,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{0, 0, 0, 0}, 17000),
	)
}

func (m *Manager) netperfTCPLocal(ctx context.Context, yourVIP net.IP) {
	// TCP Local
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		12865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{127, 0, 0, 1}, 12865),
	)
	m.dm.Upsert(ctx,
		yourVIP,
		22865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{127, 0, 0, 1}, 22865),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		12865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 12865),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		22865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 22865),
	)
}

func (m *Manager) netperfTCPRemote(ctx context.Context, yourVIP net.IP) {
	// TCP Remote
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		12865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{192, 168, 20, 3}, 12865),
	)
	m.dm.Upsert(ctx,
		yourVIP,
		22865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{192, 168, 20, 3}, 22865),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		12865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 12865),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		22865,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 22865),
	)
}

func (m *Manager) netperfUNIX(ctx context.Context, yourVIP net.IP) {
	// UNIX
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		12865,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/tiaccoon/netperf-remote.sock"),
	)
	m.dm.Upsert(ctx,
		yourVIP,
		22865,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/tiaccoon/netperf-local.sock"),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		12865,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/tiaccoon/netperf-remote.sock"),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		22865,
		destination.TransportUNIX,
		destination.NewTransportAddrUNIX("/tmp/tiaccoon/netperf-local.sock"),
	)
}

func (m *Manager) netperfRDMALocal(ctx context.Context, yourVIP net.IP) {
	// RDMA
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		12865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{192, 168, 20, 21}, 12865),
	)
	m.dm.Upsert(ctx,
		yourVIP,
		22865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{192, 168, 20, 21}, 22865),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		12865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{0, 0, 0, 0}, 12865),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		22865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{0, 0, 0, 0}, 22865),
	)
}

func (m *Manager) netperfRDMARemote(ctx context.Context, yourVIP net.IP) {
	// RDMA
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		12865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{192, 168, 20, 30}, 12865),
	)
	m.dm.Upsert(ctx,
		yourVIP,
		22865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{192, 168, 20, 30}, 22865),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		12865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{0, 0, 0, 0}, 12865),
	)
	m.dm.Upsert(ctx,
		m.myVIP,
		22865,
		destination.TransportRDMA,
		destination.NewTransportAddrRDMA([4]byte{0, 0, 0, 0}, 22865),
	)
}

func (m *Manager) nginxTCPLocal(ctx context.Context, yourVIP net.IP) {
	// TCP Local
	// ClientEntry
	m.dm.Upsert(ctx,
		yourVIP,
		80,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{127, 0, 0, 1}, 8080),
	)
	// ServerEntries
	m.dm.Upsert(ctx,
		m.myVIP,
		80,
		destination.TransportIPv4,
		destination.NewTransportAddrIPv4([4]byte{0, 0, 0, 0}, 8080),
	)
}
