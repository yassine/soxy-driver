package driver

import (
	"github.com/docker/go-plugins-helpers/network"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/netlabel"
	"github.com/fsouza/go-dockerclient"
	"net"
	"strings"
)

type InterfaceInfoProxy struct {
	request  *network.CreateEndpointRequest
	response *network.CreateEndpointResponse
	mac      net.HardwareAddr
	ip       *net.IPNet
	ip6      *net.IPNet
}

func (iface *InterfaceInfoProxy) init() {
	address, netAddress, _ := net.ParseCIDR(iface.request.Interface.Address)
	addressV6     := net.ParseIP(iface.request.Interface.AddressIPv6)
	iface.mac, _   = net.ParseMAC(iface.request.Interface.MacAddress)
  netAddress.IP  = address
	iface.ip  = netAddress
	iface.ip6 = &net.IPNet{
		IP: addressV6,
	}
}
func (iface *InterfaceInfoProxy) SetMacAddress(mac net.HardwareAddr) error {
	iface.mac = mac
	iface.response.Interface.MacAddress = mac.String()
	return nil
}
func (iface *InterfaceInfoProxy) SetIPAddress(ip *net.IPNet) error {
	iface.ip = ip
	iface.response.Interface.Address = ip.IP.String()
	return nil
}
func (iface *InterfaceInfoProxy) MacAddress() net.HardwareAddr {
	return iface.mac
}
func (iface *InterfaceInfoProxy) Address() *net.IPNet {
	return iface.ip
}
func (iface *InterfaceInfoProxy) AddressIPv6() *net.IPNet {
	return iface.ip6
}

type JoinInfoProxy struct {
	request       *network.JoinRequest
	response      *network.JoinResponse
	interfaceName InterfaceNameInfoProxy
}

func (p *JoinInfoProxy) InterfaceName() driverapi.InterfaceNameInfo {
	return p.interfaceName
}

func (p *JoinInfoProxy) SetGateway(ip net.IP) error {
	if ip != nil {
		p.response.Gateway = ip.String()
	}
	return nil
}

func (p *JoinInfoProxy) SetGatewayIPv6(ip net.IP) error {
	if ip != nil {
		p.response.GatewayIPv6 = ip.String()
	}
	return nil
}

func (p *JoinInfoProxy) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP) error {
	p.response.StaticRoutes = append(p.response.StaticRoutes, &network.StaticRoute{
		Destination: destination.String(),
		RouteType:   routeType,
		NextHop:     nextHop.String(),
	})
	return nil
}

func (p *JoinInfoProxy) DisableGatewayService() {
	p.response.DisableGatewayService = true
}

func (p *JoinInfoProxy) AddTableEntry(tableName string, key string, value []byte) error {
	return nil
}

type InterfaceNameInfoProxy struct {
	InterfaceName *network.InterfaceName
}

func (proxy InterfaceNameInfoProxy) SetNames(srcName, dstPrefix string) error {
	proxyRef := &proxy
	proxyRef.setNames(srcName, dstPrefix)
	return nil
}

func (proxy *InterfaceNameInfoProxy) setNames(srcName, dstPrefix string) error {
	proxy.InterfaceName.SrcName = srcName
	proxy.InterfaceName.DstPrefix = dstPrefix

	return nil
}

func transform(input []*network.IPAMData) []driverapi.IPAMData {
	var driverIPAM []driverapi.IPAMData
	for _, element := range input {
		gwIP, gatewayAddress, _ := net.ParseCIDR(element.Gateway)
		poolIP, poolAddress, poolError := net.ParseCIDR(element.Pool)
		gatewayAddress.IP = gwIP
		if poolError != nil {
			poolAddress.IP = poolIP
		}
		options := make(map[string]*net.IPNet)

		for key, val := range element.AuxAddresses {
			_, parsedAddress, _ := net.ParseCIDR(val.(string))
			options[key] = parsedAddress
		}
		options[bridge.DefaultGatewayV4AuxKey] = gatewayAddress

		driverIPAM = append(driverIPAM, driverapi.IPAMData{
			Gateway:      gatewayAddress,
			Pool:         poolAddress,
			AuxAddresses: options,
			AddressSpace: element.AddressSpace,
		})
	}
	return driverIPAM
}

func transformNetwork(ntwrk docker.Network) *network.CreateNetworkRequest {
	request := &network.CreateNetworkRequest{}
	request.NetworkID = ntwrk.ID

	var requestIPv4Data []*network.IPAMData
	var requestIPv6Data []*network.IPAMData
	ntwrtIPAM := ntwrk.IPAM.Config

	for _, element := range ntwrtIPAM {
		if strings.ContainsAny(element.Gateway, ".") {
			auxAddressesMap := make(map[string]interface{})

			auxAddressesMap[bridge.DefaultGatewayV4AuxKey] = element.Gateway
			requestIPv4Data = append(requestIPv4Data, &network.IPAMData{
				Gateway:      element.Gateway,
				AuxAddresses: auxAddressesMap,
				Pool:         element.Subnet,
			})
		} else if strings.ContainsAny(element.Gateway, ":") && ntwrk.EnableIPv6 {
			auxAddressesMap := make(map[string]interface{})
			auxAddressesMap[bridge.DefaultGatewayV4AuxKey] = element.Gateway
			requestIPv6Data = append(requestIPv6Data, &network.IPAMData{
				Gateway:      element.Gateway,
				AuxAddresses: auxAddressesMap,
				Pool:         element.Subnet,
			})
		}

	}

	request.IPv4Data = requestIPv4Data
	request.IPv6Data = requestIPv6Data

	options := make(map[string]interface{})
	genericOptions := make(map[string]interface{})

	for key, value := range ntwrk.Options {
		genericOptions[key] = value
	}
	options[netlabel.GenericData] = genericOptions
	request.Options = options

	return request
}
