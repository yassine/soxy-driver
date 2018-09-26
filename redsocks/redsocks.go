package redsocks

import (
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/utils"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"sync"
	"text/template"
)

//Context A base structure representing a Redsocks execution context
type Context struct {
	Command    *exec.Cmd
	Configfile *os.File
	isRunning  bool
	*Configuration
	sync.Mutex
}

//Configuration redsocks configuration params
type Configuration struct {
	//BindPort the port on which redsocks would listen to tunnel traffic
	BindPort int64
	//BindAddress the address on which redsocks would listen to tunnel traffic
	BindAddress  string
	ProxyAddress string
	//ProxyPort the proxy port
	ProxyPort int64
	//ProxyType the proxy type. Available options : as per redsocks support
	ProxyType string
	//ProxyUser the proxy user (if authentication applies)
	ProxyUser string
	//ProxyPassword the proxy password (if authentication applies)
	ProxyPassword string
	//TunnelPort the port through which traffic is tunneled
	TunnelPort int64
	//TunnelBindAddress the tunnel bind address
	TunnelBindAddress string
}

//Startup Start redsocks with the given configuration
func (r *Context) Startup() error {
	err := r.startup()
	return err
}

//Shutdown Stops redsocks with the given configuration
func (r *Context) Shutdown() error {
	err := r.shutdown()
	if err != nil {
		logrus.Error(err.Error())
	}
	return err
}

//NewContext New Creates and initialize a Redsocks context
func NewContext(configuration *Configuration) (*Context, error) {
	redsocks := &Context{
		isRunning:     false,
		Configuration: configuration,
	}
	configFile := tempFileConfig(configuration)
	command := exec.Command("redsocks", "-c", configFile.Name())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	redsocks.Command = command
	redsocks.Configfile = configFile
	return redsocks, nil
}

func (r *Context) startup() error {
	r.Lock()
	defer r.Unlock()
	if !r.isRunning {
		r.isRunning = true
		err := r.Command.Start()
		utils.LogIfNotNull(err)
	}
	return nil
}

func (r *Context) shutdown() error {
	//Kill the process
	err := r.Command.Process.Kill()
	utils.LogIfNotNull(err)
	//Remove config file
	err = os.Remove(r.Configfile.Name())
	utils.LogIfNotNull(err)
	return err
}

func (r *Context) wait() {
	r.Command.Process.Wait()
}
func tempFileConfig(configuration *Configuration) *os.File {
	t := template.Must(template.New("configTemplate").Funcs(template.FuncMap{
		"isSet":    isSet,
		"isStrSet": isStrSet,
	}).Parse(redSocksConfigurationTemplate))
	tempFile, _ := ioutil.TempFile("/tmp", "redsocks")
	err := t.Execute(tempFile, configuration)
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
    {{ if ((isStrSet .TunnelBindAddress)) }}local_ip   = {{.TunnelBindAddress}};
    {{ else }}local_ip   = 0.0.0.0;{{ end }}
    local_port = {{.TunnelPort}};
    ip         = {{.ProxyAddress}};
    port       = {{.ProxyPort}};
    {{ if (isStrSet .ProxyType) }}type   = {{.ProxyType}};
    {{ else }}type   = socks5;{{ end }}
    {{ if (isStrSet .ProxyUser) }}login   = {{.ProxyUser}};{{else}}//nothing{{ end }}
    {{ if (isStrSet .ProxyPassword) }}password   = {{.ProxyPassword}};{{else}}//nothing{{ end }}
  }

  `
