package soxy

import (
  "github.com/docker/libnetwork/iptables"
  "github.com/sirupsen/logrus"
  "github.com/yassine/soxy-driver/utils"
  "io/ioutil"
  "net"
  "os"
  "os/exec"
  "strconv"
  "sync"
  "text/template"
  "fmt"
)
const (
  IptablesSoxyChain = "SOXY_DRIVER_CHAIN"
)

const (
  BindPort      = "soxy.bindport"
  BindAddress   = "soxy.bindaddress"
  ProxyAddress  = "soxy.proxyaddress"
  ProxyPort     = "soxy.proxyport"
  ProxyType     = "soxy.proxytype"
  ProxyUser     = "soxy.proxyuser"
  ProxyPassword = "soxy.proxypassword"
)

type RedSocksConfiguration struct {
  BindPort      int64
  BindAddress   string
  ProxyAddress  string
  ProxyPort     int64
  ProxyType     string
  ProxyUser     string
  ProxyPassword string
}



type RedSocks struct {
  Configuration RedSocksConfiguration
  Command       *exec.Cmd
  Configfile    *os.File
  isRunning     bool
  bridgeName    string
  sync.Mutex
}

func (r *RedSocks) Startup() error{
  err := r.startup()
  err = r.forwardToRedSocks(iptables.Append)
  return err
}

func (r *RedSocks) Shutdown() error{
  err := r.shutdown()
  err = r.forwardToRedSocks(iptables.Delete)
  return err
}

func (r *RedSocks) forwardToRedSocks(action iptables.Action) error{
  port := r.Configuration.BindPort

  args := []string{"-t", string(iptables.Nat), string(action), IptablesSoxyChain,
    "-i", r.bridgeName,
    "-p", "tcp",
    //"--syn",
    "-j", "REDIRECT",
    "--to-ports", strconv.Itoa(int(port))}

  if output, err := iptables.Raw(args...); err != nil {
    logrus.Errorf("forwardToRedSocks error : %v", err)
  } else if len(output) != 0 {
    logrus.Errorf("forwardToRedSocks output error : %v", output)
  }

  args = []string{"-t", string(iptables.Nat), string(action), "PREROUTING",
    "-i", r.bridgeName,
    "-p", "tcp",
    "-j", IptablesSoxyChain}

  if output, err := iptables.Raw(args...); err != nil {
    return err
  } else if len(output) != 0 {
    return iptables.ChainError{Chain: "PREROUTING", Output: output}
  }


  args = []string{"-t", string(iptables.Nat),
    string(action),
    "OUTPUT",
    "-p", "tcp",
    "-j", IptablesSoxyChain}

  if output, err := iptables.Raw(args...); err != nil {
    return err
  } else if len(output) != 0 {
    return iptables.ChainError{Chain: "PREROUTING", Output: output}
  }

  return nil
}

func NewRed(params map[string]string, bridgeName string) (*RedSocks, error){
  configuration, err := newRedSocksConfiguration(params)
  if err != nil {
    return nil, err
  }
  err = configuration.validate()
  if err != nil {
    return nil, fmt.Errorf("redsocks validation error")
  }
  configFile      := tempFileConfig(&configuration)
  command := exec.Command("redsocks","-c", configFile.Name())
  command.Stdin  = os.Stdin
  command.Stdout = os.Stdout
  command.Stderr = os.Stderr

  return &RedSocks{
    Configuration : configuration,
    Configfile    : configFile,
    Command       : command,
    isRunning     : false,
    bridgeName    : bridgeName,
  }, nil
}

func(r *RedSocks) startup() error{
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

func(r *RedSocks) shutdown() error{
  //Kill the process
  err := r.Command.Process.Kill()
  //Remove config file
  err = os.Remove(r.Configfile.Name())
  return err
}

func(r *RedSocks) Wait(){
  r.Command.Process.Wait()
}

func(r RedSocksConfiguration) validate() error{
  if r.ProxyAddress == "" {
    return utils.LogAndThrowError("Proxy address is mandatory")
  }
  if r.ProxyPort == 0 {
    return utils.LogAndThrowError("Proxy port is mandatory")
  }
  return nil
}

func newRedSocksConfiguration(params map[string]string) (RedSocksConfiguration, error){
  var config = RedSocksConfiguration{}
  var err error

  for k,v := range params {
    logrus.Debugf("Param: '%s' is equal to :  '%s'",k, v)
  }

  if val, ok := params[BindPort]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", BindPort, ok)
    config.BindPort, _ = strconv.ParseInt(val, 10, 32)
    if err != nil {
      return config, utils.LogAndThrowError("error while parsing BindPort param : {}", val)
    }
  }else{
    config.BindPort = utils.FindAvailablePort()
    logrus.Debugf("No port specified , using port '%s'", config.BindPort)
  }

  if val, ok := params[BindAddress]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", BindAddress, val)
    config.BindAddress = val
    if err != nil {
      return config, utils.LogAndThrowError("error while parsing BindAddress param : {}", val)
    }
  }

  if val, ok := params[ProxyAddress]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", ProxyAddress, val)
    config.ProxyAddress = val
    if err != nil {
      return config, utils.LogAndThrowError("error while parsing ProxyAddress param : {}", val)
    }
  }else{
    config.ProxyAddress = "localhost"
  }

  if val, ok := params[ProxyPort]; ok {
    logrus.Debugf("param '%s' is equal to %s", ProxyPort, val)
    config.ProxyPort, err = strconv.ParseInt(val, 10, 32)
    if err != nil {
      return config, utils.LogAndThrowError("error while parsing ProxyPort param : {}", val)
    }
  }else{
    config.ProxyPort = 9050
  }

  if val, ok := params[ProxyType]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", ProxyType, val)
    config.ProxyType = val
  }

  if val, ok := params[ProxyPassword]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", ProxyPassword, val)
    config.ProxyPassword = val
  }

  if val, ok := params[ProxyUser]; ok {
    logrus.Debugf("param '%s' is equal to '%s'", ProxyUser, val)
    config.ProxyUser = val
  }

  return config, nil
}

func tempFileConfig(config *RedSocksConfiguration) *os.File {
  t := template.Must(template.New("configTemplate").Funcs(template.FuncMap{
    "isSet": isSet,
    "isStrSet":isStrSet,
  }).Parse(redSocksConfigurationTemplate))
  tempFile, _ := ioutil.TempFile("/tmp","redsocks")
  err := t.Execute(tempFile, config)
  if err != nil {
    logrus.Error(err)
  }
  return tempFile
}

func isSet( i net.IPAddr ) bool{
  return i.IP != nil && i.IP.String() != ""
}

func isStrSet(s string) bool {
  return s != ""
}



const redSocksConfigurationTemplate =
 `  base {
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
