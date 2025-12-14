package destination

import (
	"fmt"
	"net"
	"syscall"
)

type TransportType int32

const (
	NumTransportType = 4
)

const (
	// Priority order
	TransportUNIX TransportType = iota
	TransportRDMA
	TransportIPv6
	TransportIPv4
)

func (p TransportType) String() string {
	switch p {
	case TransportUNIX:
		return "UNIX"
	case TransportRDMA:
		return "RDMA"
	case TransportIPv6:
		return "IPv6"
	case TransportIPv4:
		return "IPv4"
	default:
		panic(fmt.Sprintf("unexpected enum %d: String() is not implemented", p))
	}
}

type TransportAddr interface {
	Byte() []byte
	String() string
}

type TransportAddrIPv4 struct {
	ip   net.IP
	port int
}

func NewTransportAddrIPv4(ip [4]byte, port int) TransportAddrIPv4 {
	return TransportAddrIPv4{
		net.IP{ip[0], ip[1], ip[2], ip[3]},
		port,
	}
}

func (t TransportAddrIPv4) Byte() []byte {
	return []byte{t.ip[0], t.ip[1], t.ip[2], t.ip[3], byte(t.port >> 8), byte(t.port)}
}

func (t TransportAddrIPv4) String() string {
	return fmt.Sprintf("%s:%d", t.ip.String(), t.port)
}

func (t TransportAddrIPv4) IP() [4]byte {
	return [4]byte{t.ip[0], t.ip[1], t.ip[2], t.ip[3]}
}

func (t TransportAddrIPv4) Port() int {
	return t.port
}

type TransportAddrUNIX struct {
	path string
}

func NewTransportAddrUNIX(path string) TransportAddrUNIX {
	return TransportAddrUNIX{path}
}

func (t TransportAddrUNIX) Byte() []byte {
	return []byte(t.path)
}

func (t TransportAddrUNIX) String() string {
	return t.path
}

func (t TransportAddrUNIX) Path() string {
	return t.path
}

type TransportAddrRDMA struct {
	family uint16
	ip     net.IP
	port   uint16
}

func NewTransportAddrRDMA(ip [4]byte, port uint16) TransportAddrRDMA {
	// TODO: support AF_INET6
	return TransportAddrRDMA{
		syscall.AF_INET,
		net.IP{ip[0], ip[1], ip[2], ip[3]},
		port,
	}
}

func (t TransportAddrRDMA) Byte() []byte {
	return []byte{t.ip[0], t.ip[1], t.ip[2], t.ip[3], byte(t.port >> 8), byte(t.port)}
}

func (t TransportAddrRDMA) String() string {
	return fmt.Sprintf("%s:%d", t.ip.String(), t.port)
}

func (t TransportAddrRDMA) Family() uint16 {
	return t.family
}

func (t TransportAddrRDMA) NetIP() net.IP {
	return t.ip
}

func (t TransportAddrRDMA) IP() [4]byte {
	return [4]byte{t.ip[0], t.ip[1], t.ip[2], t.ip[3]}
}

func (t TransportAddrRDMA) Port() uint16 {
	return t.port
}
