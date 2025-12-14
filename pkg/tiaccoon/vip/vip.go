package vip

import "net"

func IP2Int(ip net.IP) (upper uint64, lower uint64) {
	if len(ip) == 16 {
		upper = uint64(ip[0])<<56 | uint64(ip[1])<<48 | uint64(ip[2])<<40 | uint64(ip[3])<<32 | uint64(ip[4])<<24 | uint64(ip[5])<<16 | uint64(ip[6])<<8 | uint64(ip[7])
		if upper == 0 && ip[10] == 0xff && ip[11] == 0xff { // IPv4-mapped IPv6 address
			lower = uint64(ip[12])<<24 | uint64(ip[13])<<16 | uint64(ip[14])<<8 | uint64(ip[15])
		} else {
			lower = uint64(ip[8])<<56 | uint64(ip[9])<<48 | uint64(ip[10])<<40 | uint64(ip[11])<<32 | uint64(ip[12])<<24 | uint64(ip[13])<<16 | uint64(ip[14])<<8 | uint64(ip[15])
		}
		return upper, lower
	}
	return 0, uint64(ip[0])<<24 | uint64(ip[1])<<16 | uint64(ip[2])<<8 | uint64(ip[3])
}

// func Int2IP(upper, lower uint64) net.IP {
// 	return net.IP{
// 		byte(upper>>56), byte(upper>>48), byte(upper>>40), byte(upper>>32),
// 		byte(upper>>24), byte(upper>>16), byte(upper>>8), byte(upper),
// 		byte(lower>>56), byte(lower>>48), byte(lower>>40), byte(lower>>32),
// 		byte(lower>>24), byte(lower>>16), byte(lower>>8), byte(lower),
// 	}
// }
