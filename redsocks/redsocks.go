package soxy

import (
	"fmt"
	"github.com/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/utils"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

const (
	defaultChainName = "SOXY_CHAIN"
)

func IptablesSoxyChainName() string {
	if len(os.Getenv("DRIVER_NAMESPACE")) == 0 {
		return defaultChainName
	} else {
		parts := []string{strings.TrimSpace(os.Getenv("DRIVER_NAMESPACE")), defaultChainName}
		preResult := utils.GetMD5Hash(strings.Join(parts, "__"))[0:18]
		parts = []string{preResult, defaultChainName}
		return strings.Join(parts, "__")
	}
}

var (
	//IptablesSoxyChain the name of the chain as it would appear in iptables
	IptablesSoxyChain = IptablesSoxyChainName()
)

const (
	bindPort      = "soxy.bindport"
	bindAddress   = "soxy.bindaddress"
	proxyAddress  = "soxy.proxyaddress"
	proxyPort     = "soxy.proxyport"
	proxyType     = "soxy.proxytype"
	proxyUser     = "soxy.proxyuser"
	proxyPassword = "soxy.proxypassword"
)

//RedSocksConfiguration A base structure representing a Redsocks configuration context
type RedSocksConfiguration struct {
	//BindPort the port on which redsocks would listen to tunnel traffic
	BindPort int64
	//BindAddress the address on which redsocks would listen to tunnel traffic
	BindAddress string
	//ProxyAddress the proxy address
	ProxyAddress string
	//ProxyPort the proxy port
	ProxyPort int64
	//ProxyType the proxy type. Available options : as per redsocks support
	ProxyType string
	//ProxyUser the proxy user (if authentication applies)
	ProxyUser string
	//ProxyPassword the proxy password (if authentication applies)
	ProxyPassword string
	//TunnelDNS tunnel the dns resolution through tor
	TunnelDNS bool
}

//RedSocks A base structure representing a Redsocks execution context
type RedSocks struct {
	Configuration RedSocksConfiguration
	Command       *exec.Cmd
	Configfile    *os.File
	isRunning     bool
	bridgeName    string
	defaultPort   int64
	dnsPort       int64
	sync.Mutex
}

//Startup Start redsocks with the given configuration
func (r *RedSocks) Startup() error {
	err := r.forwardToRedSocks(iptables.Append)
	err = r.startup()
	return err
}

//Shutdown Stops redsocks with the given configuration
func (r *RedSocks) Shutdown() error {
	err := r.shutdown()
	err = r.forwardToRedSocks(iptables.Delete)
	if err != nil {
		logrus.Error(err.Error())
	}
	return err
}

func (r *RedSocks) forwardToRedSocks(action iptables.Action) error {
	port := r.Configuration.BindPort
	dnsPort := r.dnsPort

	//Pre-routing go to the chain
	args := []string{"-t", string(iptables.Nat), string(action), "PREROUTING",
		"-i", r.bridgeName,
		"-j", IptablesSoxyChain}

	if output, err := iptables.Raw(args...); err != nil {
		return err
	} else if len(output) != 0 {
		return iptables.ChainError{Chain: "PREROUTING", Output: output}
	}

	//TCP is redirected
	args = []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
		"-i", r.bridgeName,
		"-p", "tcp",
		"--syn",
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(int(port))}

	if output, err := iptables.Raw(args...); err != nil {
		logrus.Errorf("forwardToRedSocks error : %v", err)
	} else if len(output) != 0 {
		logrus.Errorf("forwardToRedSocks output error : %v", output)
	}

	//udp dns is redirected through tor
	args = []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
		"-i", r.bridgeName,
		"-p", "udp",
		"--dport", "53",
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(int(dnsPort)),
	}

	if output, err := iptables.Raw(args...); err != nil {
		logrus.Errorf("forwardToRedSocks DNS error : %v", err)
	} else if len(output) != 0 {
		logrus.Errorf("forwardToRedSocks DNS output error : %v", output)
	}

	args = []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
		"-i", r.bridgeName,
		"-p", "udp",
		"--dport", strconv.Itoa(int(dnsPort)),
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(int(dnsPort)),
	}

	if output, err := iptables.Raw(args...); err != nil {
		logrus.Errorf("forwardToRedSocks DNS error : %v", err)
	} else if len(output) != 0 {
		logrus.Errorf("forwardToRedSocks DNS output error : %v", output)
	}

	return nil
}

//New Creates and initialize a Redsocks context
func New(params map[string]string, bridgeName string, defaultProxyPort int64, defaultDnsPort int64) (*RedSocks, error) {
	configuration, err := newRedSocksConfiguration(params, defaultProxyPort)
	if err != nil {
		return nil, err
	}
	err = configuration.validate()
	if err != nil {
		return nil, fmt.Errorf("redsocks validation error")
	}
	configFile := tempFileConfig(&configuration)
	command := exec.Command("redsocks", "-c", configFile.Name())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return &RedSocks{
		Configuration: configuration,
		Configfile:    configFile,
		Command:       command,
		isRunning:     false,
		bridgeName:    bridgeName,
		defaultPort:   defaultProxyPort,
		dnsPort:       defaultDnsPort,
	}, nil
}

func (r *RedSocks) startup() error {
	r.Lock()
	defer r.Unlock()
	if !r.isRunning {
		r.isRunning = true
		err := r.Command.Start()
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	return nil
}

func (r *RedSocks) shutdown() error {
	//Kill the process
	err := r.Command.Process.Kill()
	//Remove config file
	err = os.Remove(r.Configfile.Name())
	return err
}

func (r *RedSocks) wait() {
	r.Command.Process.Wait()
}

func (r RedSocksConfiguration) validate() error {
	if r.ProxyAddress == "" {
		return utils.LogAndThrowError("Proxy address is mandatory")
	}
	if r.ProxyPort == 0 {
		return utils.LogAndThrowError("Proxy port is mandatory")
	}
	return nil
}

func newRedSocksConfiguration(params map[string]string, defaultProxyPort int64) (RedSocksConfiguration, error) {
	var config = RedSocksConfiguration{}
	var err error

	for k, v := range params {
		logrus.Debugf("Param: '%s' is equal to :  '%s'", k, v)
	}

	if val, ok := params[bindPort]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", bindPort, ok)
		config.BindPort, _ = strconv.ParseInt(val, 10, 32)
		if err != nil {
			return config, utils.LogAndThrowError("error while parsing BindPort param : {}", val)
		}
	} else {
		config.BindPort = utils.FindAvailablePort()
		logrus.Debugf("No port specified , using port '%s'", config.BindPort)
	}

	if val, ok := params[bindAddress]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", bindAddress, val)
		config.BindAddress = val
		if err != nil {
			return config, utils.LogAndThrowError("error while parsing BindAddress param : {}", val)
		}
	}

	if val, ok := params[proxyAddress]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", proxyAddress, val)
		config.ProxyAddress = val
		if err != nil {
			return config, utils.LogAndThrowError("error while parsing ProxyAddress param : {}", val)
		}
	} else {
		config.ProxyAddress = "localhost"
	}

	if val, ok := params[proxyPort]; ok {
		logrus.Debugf("param '%s' is equal to %s", proxyPort, val)
		config.ProxyPort, err = strconv.ParseInt(val, 10, 32)
		if err != nil {
			return config, utils.LogAndThrowError("error while parsing ProxyPort param : {}", val)
		}
	} else {
		config.ProxyPort = defaultProxyPort
	}

	if val, ok := params[proxyType]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", proxyType, val)
		config.ProxyType = val
	}

	if val, ok := params[proxyPassword]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", proxyPassword, val)
		config.ProxyPassword = val
	}

	if val, ok := params[proxyUser]; ok {
		logrus.Debugf("param '%s' is equal to '%s'", proxyUser, val)
		config.ProxyUser = val
	}

	return config, nil
}

func tempFileConfig(config *RedSocksConfiguration) *os.File {
	t := template.Must(template.New("configTemplate").Funcs(template.FuncMap{
		"isSet":    isSet,
		"isStrSet": isStrSet,
	}).Parse(redSocksConfigurationTemplate))
	tempFile, _ := ioutil.TempFile("/tmp", "redsocks")
	err := t.Execute(tempFile, config)
	if err != nil {
		logrus.Error(err)
	}
	return tempFile
}

func isSet(i net.IPAddr) bool {
	return i.IP != nil && i.IP.String() != ""
}

func isStrSet(s string) bool {
	return s != ""
}

const redSocksConfigurationTemplate = `  base {
    log_debug = off;
    log_info = on;
    log = stderr;
    daemon = off;
    redirector = iptables;
  }
  redsocks {
    {{ if ((isStrSet .BindAddress)) }}local_ip   = {{.BindAddress}};
    {{ else }}local_ip   = 0.0.0.0;{{ end }}
    local_port = {{.BindPort}};
    ip         = {{.ProxyAddress}};
    port       = {{.ProxyPort}};
    {{ if (isStrSet .ProxyType) }}type   = {{.ProxyType}};
    {{ else }}type   = socks5;{{ end }}
    {{ if (isStrSet .ProxyUser) }}login   = {{.ProxyUser}};{{else}}//nothing{{ end }}
    {{ if (isStrSet .ProxyPassword) }}password   = {{.ProxyPassword}};{{else}}//nothing{{ end }}
  }

  `
