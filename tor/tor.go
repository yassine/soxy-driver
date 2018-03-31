package tor

import (
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/utils"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"text/template"
)

//Tor a base tor structure that encapsulate the embedded tor instance
type Tor struct {
	SocksPort  int64
	command    *exec.Cmd
	configfile *os.File
	isRunning  bool
	sync.Mutex
}

//New creates and init a new Tor structure instance
func New() (t *Tor) {
	tor := &Tor{}
	tor.init()
	return tor
}

//Port returns the embedded Tor instance allocated TCP port
func (t *Tor) Port() int64 {
	return t.SocksPort
}

func (t *Tor) init() {
	t.Lock()
	defer t.Unlock()
	t.SocksPort = utils.FindAvailablePort()
	logrus.Debugf("using port '%d' as fallback tor proxy port", t.SocksPort)
	t.configfile = tempFileConfig(t)
	command := exec.Command("tor", "-f", t.configfile.Name())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	t.command = command
}

//Startup starts the embedded Tor instance
func (t *Tor) Startup() error {
	t.Lock()
	defer t.Unlock()
	if !t.isRunning {
		t.isRunning = true
		err := t.command.Start()
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	return nil
}

//Shutdown stops the embedded Tor instance
func (t *Tor) Shutdown() error {
	//Kill the process
	err := t.command.Process.Kill()
	//Remove config file
	err = os.Remove(t.configfile.Name())
	return err
}

func tempFileConfig(config *Tor) *os.File {
	t := template.Must(template.New("configTemplate").Parse(torConfigurationTemplate))
	tempFile, _ := ioutil.TempFile("/tmp", "tor-config")
	err := t.Execute(tempFile, config)
	if err != nil {
		logrus.Error(err)
	}
	return tempFile
}

const torConfigurationTemplate = `Log notice stdout
ExitPolicy reject *:*
SocksPort 0.0.0.0:{{.SocksPort}}
AutomapHostsOnResolve 1
ControlListenAddress 0.0.0.0
GeoIPExcludeUnknown 1
   `
