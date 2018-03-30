package main

import (
  "github.com/docker/go-plugins-helpers/network"
  "github.com/fsouza/go-dockerclient"
  "github.com/sirupsen/logrus"
  "github.com/yassine/soxy-driver/driver"
  "os"
  "os/signal"
  "syscall"
)

const (
  //DriverName The name of the driver (used when creating a network for example)
  DriverName   = "soxy-driver"
  //DockerSocket Docker client hook
  DockerSocket = "unix:///var/run/docker.sock"
)

func init() {
  logrus.SetLevel(logrus.DebugLevel)
}

func main() {
  client, err   := docker.NewClient(DockerSocket)
  soxyDriver    := driver.New()
  networks, err := client.ListNetworks()

  if err != nil {
    panic(err)
  }

  var recoveredNetworks []docker.Network
  for _, dockerNetwork := range networks {
    if dockerNetwork.Driver == DriverName {
      logrus.Debug(dockerNetwork.Driver, " ", dockerNetwork.Driver == DriverName)
      recoveredNetworks = append(recoveredNetworks, dockerNetwork)
    }
  }
  soxyDriver.Recover(recoveredNetworks)

  c := make(chan os.Signal, 1)
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)
  go func(){
    for sig := range c {
      if sig == syscall.SIGTERM {
        //Shutdown the driver
        soxyDriver.ShutDown()
      }
      os.Exit(0)
    }
  }()

  h := network.NewHandler(&soxyDriver)
  serveError := h.ServeUnix(DriverName, 0)
  if serveError != nil {
    logrus.Error(serveError)
  }
}