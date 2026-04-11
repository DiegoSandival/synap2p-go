package node

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func ParseAddrInfo(addr string) (peer.AddrInfo, error) {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("parse multiaddr %q: %w", addr, err)
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("extract peer info from %q: %w", addr, err)
	}
	return *info, nil
}

func ParseAddrInfos(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		info, err := ParseAddrInfo(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}