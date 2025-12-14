package seccomp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
)

type sockaddr struct {
	syscall.RawSockaddr
	IP       net.IP
	Port     uint16
	Flowinfo uint32 // sin6_flowinfo
	ScopeID  uint32 // sin6_scope_id
}

func (sa *sockaddr) String() string {
	return fmt.Sprintf("%s:%d", sa.IP, sa.Port)
}

func newSockaddr(buf []byte) (*sockaddr, error) {
	sa := &sockaddr{}
	reader := bytes.NewReader(buf)
	// TODO: support big endian hosts
	endian := binary.LittleEndian
	if err := binary.Read(reader, endian, &sa.RawSockaddr); err != nil {
		return nil, fmt.Errorf("cannot cast byte array to RawSocksddr: %w", err)
	}
	switch sa.Family {
	case syscall.AF_INET:
		addr4 := syscall.RawSockaddrInet4{}
		if _, err := reader.Seek(0, 0); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, endian, &addr4); err != nil {
			return nil, fmt.Errorf("cannot cast byte array to RawSockaddrInet4: %w", err)
		}
		sa.IP = make(net.IP, len(addr4.Addr))
		copy(sa.IP, addr4.Addr[:])
		p := make([]byte, 2)
		binary.BigEndian.PutUint16(p, addr4.Port)
		sa.Port = endian.Uint16(p)
	case syscall.AF_INET6:
		addr6 := syscall.RawSockaddrInet6{}
		if _, err := reader.Seek(0, 0); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, endian, &addr6); err != nil {
			return nil, fmt.Errorf("cannot cast byte array to RawSockaddrInet6: %w", err)
		}
		sa.IP = make(net.IP, len(addr6.Addr))
		copy(sa.IP, addr6.Addr[:])
		p := make([]byte, 2)
		binary.BigEndian.PutUint16(p, addr6.Port)
		sa.Port = endian.Uint16(p)
		sa.Flowinfo = addr6.Flowinfo
		sa.ScopeID = addr6.Scope_id
	default:
		return nil, fmt.Errorf("expected AF_INET or AF_INET6, got %d", sa.Family)
	}
	return sa, nil
}

func newSockAddrFromIPPort(domain uint16, ip net.IP, port uint16, flowinfo, scopeID uint32) (*sockaddr, error) {
	sa := &sockaddr{}
	sa.Family = domain
	switch sa.Family {
	case syscall.AF_INET:
		sa.IP = make(net.IP, len(ip.To4()))
		copy(sa.IP, ip.To4())
		sa.Port = port
	case syscall.AF_INET6:
		sa.IP = make(net.IP, len(ip.To16()))
		copy(sa.IP, ip.To16())
		sa.Port = port
		sa.Flowinfo = flowinfo
		sa.ScopeID = scopeID
	default:
		return nil, fmt.Errorf("expected AF_INET or AF_INET6, got %d", sa.Family)
	}
	return sa, nil
}

func sockaddrToByte(sa *sockaddr) ([]byte, error) {
	buf := new(bytes.Buffer)
	endian := binary.LittleEndian

	switch sa.Family {
	case syscall.AF_INET:
		addr4 := syscall.RawSockaddrInet4{
			Family: sa.Family,
			Port:   binary.BigEndian.Uint16([]byte{byte(sa.Port & 0xff), byte(sa.Port >> 8)}),
		}
		copy(addr4.Addr[:], sa.IP.To4())
		if err := binary.Write(buf, endian, addr4); err != nil {
			return nil, fmt.Errorf("cannot write RawSockaddrInet4 to buffer: %w", err)
		}
	case syscall.AF_INET6:
		addr6 := syscall.RawSockaddrInet6{
			Family:   sa.Family,
			Port:     binary.BigEndian.Uint16([]byte{byte(sa.Port & 0xff), byte(sa.Port >> 8)}),
			Flowinfo: sa.Flowinfo,
			Scope_id: sa.ScopeID,
		}
		copy(addr6.Addr[:], sa.IP.To16())
		if err := binary.Write(buf, endian, addr6); err != nil {
			return nil, fmt.Errorf("cannot write RawSockaddrInet6 to buffer: %w", err)
		}
	default:
		return nil, fmt.Errorf("expected AF_INET or AF_INET6, got %d", sa.Family)
	}

	return buf.Bytes(), nil
}

func zeroSockaddr() *sockaddr {
	sa := &sockaddr{}
	sa.Family = syscall.AF_INET // TODO: Set by handling the syscall socket
	sa.IP = net.IPv4zero
	sa.Port = 0
	sa.Flowinfo = 0
	sa.ScopeID = 0
	return sa
}
