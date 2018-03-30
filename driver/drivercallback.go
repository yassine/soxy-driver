package driver

import (
  "github.com/docker/docker/pkg/plugingetter"
  "github.com/docker/libnetwork/driverapi"
)

type Callback struct {
  driver driverapi.Driver
}

func (*Callback) GetPluginGetter() plugingetter.PluginGetter {
  return nil
}
// RegisterDriver provides a way for Remote drivers to dynamically register new NetworkType and associate with a driver instance
func (d *Callback) RegisterDriver(name string, driver driverapi.Driver, capability driverapi.Capability) error {
  d.driver = driver
  return nil
}

