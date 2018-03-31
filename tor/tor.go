package tor

import (
  "os"
  "sync"
  "os/exec"
  "github.com/yassine/soxy-driver/utils"
  "text/template"
  "io/ioutil"
  "github.com/sirupsen/logrus"
)

type Tor struct {
  port       int64
  command    *exec.Cmd
  configfile *os.File
  isRunning  bool
  sync.Mutex
}

func New() (t *Tor) {
  tor := &Tor{}
  tor.init()
  return tor
}

func (t *Tor) Port() (int64) {
  return t.port
}

func (t *Tor) init(){
  t.Lock()
  defer t.Unlock()
  t.port       = utils.FindAvailablePort()
  logrus.Debugf("using port '%d' as fallback tor proxy port", t.port)
  t.configfile = tempFileConfig(t)
  command := exec.Command("tor","-f", t.configfile.Name())
  command.Stdin  = os.Stdin
  command.Stdout = os.Stdout
  command.Stderr = os.Stderr
  t.command = command
}
func(t *Tor) Startup() error{
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

func(t *Tor) Shutdown() error{
  //Kill the process
  err := t.command.Process.Kill()
  //Remove config file
  err = os.Remove(t.configfile.Name())
  return err
}

func tempFileConfig(config *Tor) *os.File {
  t := template.Must(template.New("configTemplate").Funcs(template.FuncMap{
    "TorSocksPort": config.port,
  }).Parse(torConfigurationTemplate))
  tempFile, _ := ioutil.TempFile("/tmp","tor-config")
  err := t.Execute(tempFile, config)
  if err != nil {
    logrus.Error(err)
  }
  return tempFile
}


const torConfigurationTemplate =
  `Log notice stdout
ExitPolicy reject *:*
SocksPort 0.0.0.0:{{.TorSocksPort}}
AutomapHostsOnResolve 1
ControlListenAddress 0.0.0.0
GeoIPExcludeUnknown 1
   `
