package network

import (
	"github.com/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/redsocks"
	"github.com/yassine/soxy-driver/utils"
	"strconv"
)

const (
	proxyAddress      = "soxy.proxyaddress"
	proxyPort         = "soxy.proxyport"
	proxyType         = "soxy.proxytype"
	proxyUser         = "soxy.proxyuser"
	proxyPassword     = "soxy.proxypassword"
	tunnelBindAddress = "soxy.tunnelBindAddress"
	tunnelPort        = "soxy.tunnelPort"
	blockUDP          = "soxy.blockUDP"
	defaultChainName  = "SOXY_CHAIN"
)

//IptablesSoxyChain The Soxy driver custom iptables chain name
var IptablesSoxyChain = iptablesSoxyChainName()

//Context encapsulates network configuration
type Context struct {
	// The network id
	ID string
	// The associated linux network bridge name
	BridgeName string
	//ProxyPort the proxy address
	ProxyAddress string
	//ProxyPort the proxy port
	ProxyPort int64
	//ProxyPassword the proxy password (if authentication applies)
	ProxyPassword string
	//ProxyType the proxy type. Available options : as per redsocks support
	ProxyType string
	//ProxyUser the proxy user (if authentication applies)
	ProxyUser string
	//TunnelBindAddress the tunnel bind address
	TunnelBindAddress string
	//TunnelDNS tunnel the dns resolution through tor
	TunnelDNS bool
	//TunnelPort the port through which traffic is tunneled
	TunnelPort int64
	//TunnelDNSPort tunnel the dns tunnel port
	TunnelDNSPort int64
	//TunnelDNS tunnel the dns resolution through tor
	BlockUDP bool
	//Redsocks The redsocks context associated with a given network
	redsocks *redsocks.Context
}

//NewContext returns a new network context
func NewContext(networkID string, bridgeName string, params map[string]string, defaultProxyPort int64, dnsPort int64) (*Context, error) {

	networkContext := &Context{
		ID:            networkID,
		BridgeName:    bridgeName,
		TunnelDNSPort: dnsPort,
	}
	err := parseNetworkConfiguration(networkContext, params, defaultProxyPort)

	if err != nil {
		return nil, err
	}

	err = preconditions(networkContext)

	if err != nil {
		return nil, err
	}

	redsocksContext, redsocksError := redsocks.NewContext(buildRedsocksConfig(networkContext))

	if redsocksError != nil {
		return nil, err
	}

	networkContext.redsocks = redsocksContext

	return networkContext, nil
}

//Init initialize the network context
func (networkContext *Context) Init() error {
	err := networkContext.programNetworkIfaceRules(iptables.Append)
	if err != nil {
		logrus.Error(err.Error())
	}
	err = networkContext.redsocks.Startup()
	if err != nil {
		logrus.Error(err.Error())
	}
	return err
}

//Cleanup cleans-up the network context
func (networkContext *Context) Cleanup() error {
	err := networkContext.programNetworkIfaceRules(iptables.Delete)
	if err != nil {
		logrus.Error(err.Error())
	}
	err = networkContext.redsocks.Shutdown()
	if err != nil {
		logrus.Error(err.Error())
	}
	return err
}

func buildRedsocksConfig(networkContext *Context) *redsocks.Configuration {
	return &redsocks.Configuration{
		ProxyAddress:      networkContext.ProxyAddress,
		ProxyPassword:     networkContext.ProxyPassword,
		ProxyPort:         networkContext.ProxyPort,
		ProxyType:         networkContext.ProxyType,
		ProxyUser:         networkContext.ProxyUser,
		TunnelBindAddress: networkContext.TunnelBindAddress,
		TunnelPort:        networkContext.TunnelPort,
	}
}

func (networkContext *Context) programNetworkIfaceRules(action iptables.Action) error {

	/**********************
	 ****** Routing *******
	 **********************/

	//Pre-routing: go to the chain
	args := []string{"-t", string(iptables.Nat), string(action), "PREROUTING",
		"-i", networkContext.BridgeName,
		"-j", IptablesSoxyChain}

	if output, err := iptables.Raw(args...); err != nil {
		return err
	} else if len(output) != 0 {
		return iptables.ChainError{Chain: "PREROUTING", Output: output}
	}

	//udp dns is redirected through tor
	args = []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
		"-i", networkContext.BridgeName,
		"-p", "udp",
		"--dport", "53",
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(int(networkContext.TunnelDNSPort)),
	}

	if output, err := iptables.Raw(args...); err != nil {
		logrus.Errorf("forwardToRedSocks DNS error : %v", err)
	} else if len(output) != 0 {
		logrus.Errorf("forwardToRedSocks DNS output error : %v", output)
	}

	//TCP traffic is redirected through the tunnel
	args = []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
		"-i", networkContext.BridgeName,
		"-p", "tcp",
		"--syn",
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(int(networkContext.TunnelPort))}

	if output, err := iptables.Raw(args...); err != nil {
		logrus.Errorf("forwardToRedSocks error : %v", err)
	} else if len(output) != 0 {
		logrus.Errorf("forwardToRedSocks output error : %v", output)
	}

	/*************************
	 ******* Filtering *******
	 *************************/

	if networkContext.BlockUDP {

		altAction := action
		if action == iptables.Append {
			altAction = iptables.Insert
		}

		//FORWARD

		args := []string{"-t", string(iptables.Filter), string(altAction), "FORWARD",
			"-i", networkContext.BridgeName,
			"-j", IptablesSoxyChain}

		if output, err := iptables.Raw(args...); err != nil {
			logrus.Errorf("error '%s' while filter -i forward", err.Error())
			return err
		} else if len(output) != 0 {
			logrus.Errorf("error '%s' while filter -i forward", err.Error())
			return iptables.ChainError{Chain: "FORWARD", Output: output}
		}

		args = []string{"-t", string(iptables.Filter), string(action), IptablesSoxyChain,
			"-i", networkContext.BridgeName,
			"-p", "udp",
			"--dport", strconv.Itoa(int(networkContext.TunnelDNSPort)),
			"-j", "RETURN",
		}

		if output, err := iptables.Raw(args...); err != nil {
			logrus.Errorf("forwardToRedSocks Authorizing DNS traffic error : %v", err)
		} else if len(output) != 0 {
			logrus.Errorf("forwardToRedSocks Authorizing DNS traffic output error : %v", output)
		}

		args = []string{"-t", string(iptables.Filter), string(action), IptablesSoxyChain,
			"-i", networkContext.BridgeName,
			"-p", "udp",
			"-j", "DROP",
		}
		if output, err := iptables.Raw(args...); err != nil {
			logrus.Errorf("forwardToRedSocks Blocking UDP traffic error : %v", err)
		} else if len(output) != 0 {
			logrus.Errorf("forwardToRedSocks Blocking UDP traffic output error : %v", output)
		}

	}

	return nil
}
func parseNetworkConfiguration(networkContext *Context, params map[string]string, defaultProxyPort int64) error {

	var err error

	for k, v := range params {
		logrus.Debugf("Param: '%s' is equal to :  '%s'", k, v)
	}

	if val, ok := params[proxyAddress]; ok {
		networkContext.ProxyAddress = val
		if err != nil {
			return utils.LogAndThrowError("error while parsing ProxyAddress param : {}", val)
		}
	} else {
		networkContext.ProxyAddress = "localhost"
	}

	if val, ok := params[proxyPort]; ok {
		networkContext.ProxyPort, err = strconv.ParseInt(val, 10, 32)
		if err != nil {
			return utils.LogAndThrowError("error while parsing ProxyPort param : {}", val)
		}
	} else {
		networkContext.ProxyPort = defaultProxyPort
	}

	if val, ok := params[proxyType]; ok {
		networkContext.ProxyType = val
	}

	if val, ok := params[proxyUser]; ok {
		networkContext.ProxyUser = val
	}

	if val, ok := params[proxyPassword]; ok {
		networkContext.ProxyPassword = val
	}

	if val, ok := params[tunnelBindAddress]; ok {
		networkContext.TunnelBindAddress = val
	}

	if val, ok := params[tunnelPort]; ok {
		networkContext.TunnelPort, err = strconv.ParseInt(val, 10, 32)
		if err != nil {
			logrus.Warningf("error while parsing param '%s' :found value '%s'", tunnelPort, val)
			networkContext.TunnelPort = utils.FindAvailablePort()
		}
	} else {
		networkContext.TunnelPort = utils.FindAvailablePort()
	}

	if val, ok := params[blockUDP]; ok {
		b, err := strconv.ParseBool(params[blockUDP])
		if err != nil {
			logrus.Warningf("param '%s' is invalid boolean '%s'", blockUDP, val)
			networkContext.BlockUDP = true
		} else {
			networkContext.BlockUDP = b
		}
	}

	return nil
}
func preconditions(networkContext *Context) error {
	if networkContext.ProxyAddress == "" {
		return utils.LogAndThrowError("Proxy address is mandatory")
	}
	if networkContext.ProxyPort == 0 {
		return utils.LogAndThrowError("Proxy port is mandatory")
	}
	return nil
}
