package driver

import (
	"github.com/docker/go-plugins-helpers/network"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

const (
	fixtureNetworkIP = "172.21.1.1/24"
	fixtureMac       = "10:10:10:10:10:10"
	fixtureIpv6      = "fe80::800:27ff:fe00:0"
)

func TestInterfaceInfoProxyInit(t *testing.T) {
	address, _, _ := net.ParseCIDR(fixtureNetworkIP)
	proxy := createInterface()
	assert.Equal(t, proxy.ip.IP, address)
}

func TestInterfaceInfoProxySetMacAddress(t *testing.T) {
	iface := createInterface()
	macAddr, _ := net.ParseMAC(fixtureMac)
	iface.SetMacAddress(macAddr)
	assert.Equal(t, iface.response.Interface.MacAddress, fixtureMac)
}

func TestInterfaceInfoProxySetIPAddress(t *testing.T) {
	iface := createInterface()
	_, netAddress, _ := net.ParseCIDR(fixtureNetworkIP)
	iface.SetIPAddress(netAddress)
	assert.Equal(t, iface.response.Interface.Address, netAddress.IP.String())
}

func TestInterfaceInfoProxyMacAddress(t *testing.T) {
	iface := createInterface()
	assert.Equal(t, iface.mac.String(), fixtureMac)
}

func TestInterfaceInfoProxyAddress(t *testing.T) {
	iface := createInterface()
	address, _, _ := net.ParseCIDR(fixtureNetworkIP)
	assert.Equal(t, iface.ip.IP, address)
}

func TestInterfaceInfoProxyV6Address(t *testing.T) {
	iface := createInterface()
	address := net.ParseIP(fixtureIpv6)
	assert.Equal(t, iface.ip6.IP, address)
}

func createInterface() *InterfaceInfoProxy {
	proxy := &InterfaceInfoProxy{
		request: mockRequest(),
		response: &network.CreateEndpointResponse{
			Interface: &network.EndpointInterface{},
		},
	}
	proxy.init()
	return proxy
}

func mockRequest() *network.CreateEndpointRequest {
	return &network.CreateEndpointRequest{
		NetworkID:  "NT0000",
		EndpointID: "EP0000",
		Options:    make(map[string]interface{}),
		Interface: &network.EndpointInterface{
			Address:     fixtureNetworkIP,
			MacAddress:  fixtureMac,
			AddressIPv6: fixtureIpv6,
		},
	}
}
