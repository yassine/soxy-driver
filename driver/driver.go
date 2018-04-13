package driver

import (
	"fmt"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/redsocks"
	"github.com/yassine/soxy-driver/tor"
	"net"
)

//Driver A Driver structure
type Driver struct {
	delegate      *driverapi.Driver
	redsocksIndex map[string]*soxy.RedSocks
	tor           *tor.Tor
}

//New Creates a new Driver instance
func New() Driver {
	driverCallback := &Callback{}
	var bridgeDriverOptions = make(map[string]interface{})
	genericOptions := make(options.Generic)
	genericOptions["EnableIPForwarding"] = true
	genericOptions["EnableIPTables"] = true
	genericOptions["EnableUserlandProxy"] = false
	bridgeDriverOptions[netlabel.GenericData] = genericOptions
	err := bridge.Init(driverCallback, bridgeDriverOptions)
	if err != nil {
		logrus.Error(err.Error())
	}
	driver := Driver{delegate: &driverCallback.driver, tor: tor.New(), redsocksIndex: make(map[string]*soxy.RedSocks)}
	driver.init()
	return driver
}

//GetCapabilities driver-utils contract implementation
func (d *Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	logrus.Debug("Received Get Capabilities Request")
	return &network.CapabilitiesResponse{Scope: network.LocalScope}, nil
}

//CreateNetwork driver-utils contract implementation
func (d *Driver) CreateNetwork(request *network.CreateNetworkRequest) error {
	logrus.Debug("Received Get CreateNetwork Request : ", request.NetworkID)
	delegate := *d.delegate
	ipv4Addresses := transform(request.IPv4Data)
	ipv6Addresses := transform(request.IPv6Data)
	err := delegate.CreateNetwork(request.NetworkID, parseNetworkOptions(request.Options), nil, ipv4Addresses, ipv6Addresses)
	link, _ := findLinkByAddress(ipv4Addresses[0].Gateway.IP.String())
	if link != nil {
		allocatedBridgeName := link.Attrs().Name
		logrus.Debug("Allocated the bridge : ", allocatedBridgeName, " to network : ", request.NetworkID)
		redsocksContext, redsocksError := soxy.New(request.Options[netlabel.GenericData].(map[string]string), allocatedBridgeName, d.tor.Port(), d.tor.DnsPort)
		if redsocksError == nil {
			d.redsocksIndex[request.NetworkID] = redsocksContext
			redsocksContext.Startup()
		} else {
			logrus.Error(redsocksError.Error())
			return redsocksError
		}
	}
	return err
}

//AllocateNetwork driver-utils contract implementation
func (d *Driver) AllocateNetwork(request *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	delegate := *d.delegate
	_, err := delegate.NetworkAllocate(request.NetworkID, nil, nil, nil)
	return nil, err
}

//DeleteNetwork driver-utils contract implementation
func (d *Driver) DeleteNetwork(request *network.DeleteNetworkRequest) error {
	logrus.Debug("Received Get DeleteNetwork Request : %s", request.NetworkID)
	delegate := *d.delegate
	err := d.redsocksIndex[request.NetworkID].Shutdown()
	if err != nil {
		return err
	}
	return delegate.DeleteNetwork(request.NetworkID)
}

//FreeNetwork driver-utils contract implementation
func (d *Driver) FreeNetwork(request *network.FreeNetworkRequest) error {
	delegate := *d.delegate
	return delegate.NetworkFree(request.NetworkID)
}

//CreateEndpoint driver-utils contract implementation
func (d *Driver) CreateEndpoint(request *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	delegate := *d.delegate
	proxy := &InterfaceInfoProxy{
		request: request,
		response: &network.CreateEndpointResponse{
			Interface: &network.EndpointInterface{},
		},
	}
	proxy.init()
	err := delegate.CreateEndpoint(request.NetworkID, request.EndpointID, proxy, request.Options)
	return proxy.response, err
}

//DeleteEndpoint driver-utils contract implementation
func (d *Driver) DeleteEndpoint(request *network.DeleteEndpointRequest) error {
	logrus.Debug("Received DeleteEndpoint Request %s @ %s", request.EndpointID, request.NetworkID)
	delegate := *d.delegate
	return delegate.DeleteEndpoint(request.NetworkID, request.EndpointID)
}

//EndpointInfo driver-utils contract implementation
func (d *Driver) EndpointInfo(request *network.InfoRequest) (*network.InfoResponse, error) {
	logrus.Debug("Received EndpointInfo Request %s @ %s", request.EndpointID, request.NetworkID)
	delegate := *d.delegate
	info, _ := delegate.EndpointOperInfo(request.NetworkID, request.EndpointID)
	m := map[string]string{}
	if info[netlabel.ExposedPorts] != nil {
		var exposedPorts []types.TransportPort
		var exposedPortsString = ""
		exposedPorts = info[netlabel.ExposedPorts].([]types.TransportPort)
		for _, port := range exposedPorts {
			exposedPortsString += port.String()
		}
	}
	if info[netlabel.MacAddress] != nil {
		m[netlabel.MacAddress] = info[netlabel.MacAddress].(net.HardwareAddr).String()
	}
	return &network.InfoResponse{
		Value: m,
	}, nil
}

//Join driver-utils contract implementation
func (d *Driver) Join(request *network.JoinRequest) (*network.JoinResponse, error) {
	logrus.Debug("Received Join Request %s @ %s", request.EndpointID, request.NetworkID)
	delegate := *d.delegate

	ifaceNameProxy := InterfaceNameInfoProxy{
		InterfaceName: &network.InterfaceName{},
	}
	joinInfoProxy := &JoinInfoProxy{
		request: request,
		response: &network.JoinResponse{
			StaticRoutes: []*network.StaticRoute{},
		},
		interfaceName: ifaceNameProxy,
	}

	err := delegate.Join(request.NetworkID, request.EndpointID, request.SandboxKey, joinInfoProxy, request.Options)

	joinInfoProxy.response.InterfaceName.SrcName = ifaceNameProxy.InterfaceName.SrcName
	joinInfoProxy.response.InterfaceName.DstPrefix = ifaceNameProxy.InterfaceName.DstPrefix

	return joinInfoProxy.response, err
}

//Leave driver-utils contract implementation
func (d *Driver) Leave(request *network.LeaveRequest) error {
	logrus.Debug("Received Leave Request %s @ %s", request.EndpointID, request.NetworkID)
	delegate := *d.delegate
	return delegate.Leave(request.NetworkID, request.EndpointID)
}

//DiscoverNew driver-utils contract implementation
func (d *Driver) DiscoverNew(request *network.DiscoveryNotification) error {
	return fmt.Errorf("not supported")
}

//DiscoverDelete driver-utils contract implementation
func (d *Driver) DiscoverDelete(request *network.DiscoveryNotification) error {
	return fmt.Errorf("not supported")
}

//ProgramExternalConnectivity driver-utils contract implementation
func (d *Driver) ProgramExternalConnectivity(request *network.ProgramExternalConnectivityRequest) error {
	delegate := *d.delegate
	logrus.Debug("Received ProgramExternalConnectivity Request")

	var mappings []types.PortBinding
	var rawMapping = request.Options[netlabel.PortMap].([]interface{})
	for _, element := range rawMapping {
		element2 := element.(map[string]interface{})
		mappings = append(mappings, types.PortBinding{
			IP:          net.ParseIP(element2["IP"].(string)),
			Proto:       protocolValueOf(uint8(element2["Proto"].(float64))),
			Port:        uint16(element2["Port"].(float64)),
			HostIP:      net.ParseIP(element2["HostIP"].(string)),
			HostPort:    uint16(element2["HostPort"].(float64)),
			HostPortEnd: uint16(element2["HostPortEnd"].(float64)),
		})
	}

	var exposedPorts []types.TransportPort
	var rawExposedPorts = request.Options[netlabel.ExposedPorts].([]interface{})
	for _, element := range rawExposedPorts {
		element2 := element.(map[string]interface{})
		exposedPorts = append(exposedPorts, types.TransportPort{
			Proto: protocolValueOf(uint8(element2["Proto"].(float64))),
			Port:  uint16(element2["Port"].(float64)),
		})
	}

	var opts = make(map[string]interface{})
	opts[netlabel.PortMap] = mappings
	opts[netlabel.ExposedPorts] = exposedPorts

	return delegate.ProgramExternalConnectivity(request.NetworkID, request.EndpointID, opts)
}

//RevokeExternalConnectivity driver-utils contract implementation
func (d *Driver) RevokeExternalConnectivity(request *network.RevokeExternalConnectivityRequest) error {
	logrus.Debug("Received RevokeExternalConnectivity Request")
	delegate := *d.delegate
	return delegate.RevokeExternalConnectivity(request.NetworkID, request.EndpointID)
}

//Recover updates in-memory information on driver startup (e.g. if networks using the driver already exist)
func (d *Driver) Recover(networks []docker.Network) {
	for _, element := range networks {
		logrus.Debug("Recovering network ... ", element.ID)
		err := d.CreateNetwork(transformNetwork(element))
		if err != nil {
			logrus.Error("Failed while recovering network : ", element.ID)
			logrus.Error(err)
		}
	}
}

//ShutDown shutdown hook, used to free resources
func (d *Driver) ShutDown() {
	for _, value := range d.redsocksIndex {
		value.Shutdown()
	}
	d.removeChain()
	(*d.tor).Shutdown()
}

func (d *Driver) init() {
	d.createChain()
	(*d.tor).Startup()
}

func (d *Driver) createChain() error {
	//create SOXYDRIVER CHAIN
	logrus.Debug("creating soxy-driver chain")
	err := createChain(iptables.Nat, true)
	if err != nil {
		return err
	}
	err = createChain(iptables.Filter, true)
	return err
}

func createChain(table iptables.Table, escapeLocal bool) error {
	args := []string{"-t", string(table), "-N", soxy.IptablesSoxyChain}
	if output, err := iptables.Raw(args...); err != nil || len(output) != 0 {
		logrus.Debug(fmt.Errorf("couldn't setup soxychain chain in table '%s' : %s", table, err).Error())
	} else {
		//escape local addresses
		if escapeLocal {
			for _, address := range LocalAddresses {
				args = []string{"-t", string(table), string(iptables.Append), soxy.IptablesSoxyChain,
					"-d", address,
					"-j", "RETURN"}
				if output, err := iptables.Raw(args...); err != nil || len(output) != 0 {
					logrus.Errorf("couldn't setup in table %s soxychain local addresss escape : %v", address, table)
					return err
				}
			}
		}
	}
	return nil
}

func (d *Driver) removeChain() {
	iptables.RemoveExistingChain(soxy.IptablesSoxyChain, iptables.Nat)
	iptables.RemoveExistingChain(soxy.IptablesSoxyChain, iptables.Filter)
}
