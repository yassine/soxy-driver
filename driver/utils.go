package driver

import (
	"fmt"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
  soxyNetwork "github.com/yassine/soxy-driver/network"
	"net"
)

func findLinkByAddress(address string) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		addresses, _ := netlink.AddrList(link, netlink.FAMILY_ALL)
		for _, addr := range addresses {
			if addr.Contains(net.ParseIP(address)) {
				return link, nil
			}
			// TODO IPv6?
		}
	}
	return nil, fmt.Errorf("link having address '%s' not found", address)
}

func parseNetworkOptions(data map[string]interface{}) map[string]interface{} {
	if genData, ok := data[netlabel.GenericData]; ok && genData != nil {
		result := make(map[string]string)
		for key, value := range data[netlabel.GenericData].(map[string]interface{}) {
			result[key] = value.(string)
		}
		data[netlabel.GenericData] = result
	}
	return data
}

func protocolValueOf(val uint8) types.Protocol {
	if val == types.TCP {
		return types.TCP
	}
	if val == types.UDP {
		return types.UDP
	}
	if val == types.ICMP {
		return types.ICMP
	}
	return types.ICMP
}

func createChain(table iptables.Table, escapeLocal bool) error {
	args := []string{"-t", string(table), "-N", soxyNetwork.IptablesSoxyChain}
	if output, err := iptables.Raw(args...); err != nil || len(output) != 0 {
		logrus.Debug(fmt.Errorf("couldn't setup soxychain chain in table '%s' : %s", table, err).Error())
	}
  if escapeLocal {
    for _, address := range LocalAddresses {
      args = []string{"-t", string(table), string(iptables.Insert), soxyNetwork.IptablesSoxyChain,
        "-d", address,
        "-j", "RETURN"}
      if output, err := iptables.Raw(args...); err != nil || len(output) != 0 {
        logrus.Errorf("couldn't setup in table %s soxychain local addresss escape : %v", address, table)
        return err
      }
    }
  }
	return nil
}

var (
	//LocalAddresses reserved local addresses
	LocalAddresses = []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"224.0.0.0/4",
		"240.0.0.0/4",
	}
)
