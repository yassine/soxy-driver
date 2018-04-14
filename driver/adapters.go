package driver

import (
	"github.com/docker/go-plugins-helpers/network"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/netlabel"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"net"
	"strings"
)

//InterfaceInfoProxy a libnetwork InfoProxy proxy
type InterfaceInfoProxy struct {
	request  *network.CreateEndpointRequest
	response *network.CreateEndpointResponse
	mac      net.HardwareAddr
	ip       *net.IPNet
	ip6      *net.IPNet
}

func (iface *InterfaceInfoProxy) init() {
	address, netAddress, _ := net.ParseCIDR(iface.request.Interface.Address)
	addressV6 := net.ParseIP(iface.request.Interface.AddressIPv6)
	iface.mac, _ = net.ParseMAC(iface.request.Interface.MacAddress)
	netAddress.IP = address
	iface.ip = netAddress
	iface.ip6 = &net.IPNet{
		IP: addressV6,
	}
}

//SetMacAddress intercepts the SetMacAddress call and updates the response data
func (iface *InterfaceInfoProxy) SetMacAddress(mac net.HardwareAddr) error {
	iface.mac = mac
	iface.response.Interface.MacAddress = mac.String()
	return nil
}

//SetIPAddress intercepts the SetIPAddress call and updates the response data
func (iface *InterfaceInfoProxy) SetIPAddress(ip *net.IPNet) error {
	iface.ip = ip
	iface.response.Interface.Address = ip.IP.String()
	return nil
}

//MacAddress returns the Mac address of the network interface
func (iface *InterfaceInfoProxy) MacAddress() net.HardwareAddr {
	return iface.mac
}

//Address returns the network address of the network interface
func (iface *InterfaceInfoProxy) Address() *net.IPNet {
	return iface.ip
}

//AddressIPv6 returns the network address (v6) of the network interface
func (iface *InterfaceInfoProxy) AddressIPv6() *net.IPNet {
	return iface.ip6
}

//JoinInfoProxy a libnetwork JoinInfo proxy
type JoinInfoProxy struct {
	request       *network.JoinRequest
	response      *network.JoinResponse
	interfaceName InterfaceNameInfoProxy
}

//InterfaceName returns the network interface name
func (p *JoinInfoProxy) InterfaceName() driverapi.InterfaceNameInfo {
	return p.interfaceName
}

//SetGateway intercepts the libnetwork SetGateway call and updates the driver response
func (p *JoinInfoProxy) SetGateway(ip net.IP) error {
	if ip != nil {
		p.response.Gateway = ip.String()
	}
	return nil
}

//SetGatewayIPv6 intercepts the libnetwork SetGatewayIPv6 call and updates the driver response
func (p *JoinInfoProxy) SetGatewayIPv6(ip net.IP) error {
	if ip != nil {
		p.response.GatewayIPv6 = ip.String()
	}
	return nil
}

//AddStaticRoute intercepts the libnetwork AddStaticRoute call and updates the driver response
func (p *JoinInfoProxy) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP) error {
	p.response.StaticRoutes = append(p.response.StaticRoutes, &network.StaticRoute{
		Destination: destination.String(),
		RouteType:   routeType,
		NextHop:     nextHop.String(),
	})
	return nil
}

//DisableGatewayService intercepts the libnetwork DisableGatewayService call and updates the driver response
func (p *JoinInfoProxy) DisableGatewayService() {
	p.response.DisableGatewayService = true
}

//AddTableEntry unsupported
func (p *JoinInfoProxy) AddTableEntry(tableName string, key string, value []byte) error {
	return nil
}

//InterfaceNameInfoProxy a libnetwork InterfaceNameInfo proxy
type InterfaceNameInfoProxy struct {
	InterfaceName *network.InterfaceName
}

//SetNames intercepts the libnetwork SetNames
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
		gwIP, gatewayAddress, err := net.ParseCIDR(element.Gateway)
		if err != nil {
			gwIP, gatewayAddress, err = net.ParseCIDR(element.Gateway + "/24")
			if err != nil {
				logrus.Error(err.Error())
			}
		}
		gatewayAddress.IP = gwIP
		poolIP, poolAddress, poolError := net.ParseCIDR(element.Pool)

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
