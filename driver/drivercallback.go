package driver

import (
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/libnetwork/driverapi"
)

//Callback a callback implementation used to highjack a libnetwork bridge driver instance
type Callback struct {
	driver driverapi.Driver
}

//GetPluginGetter not supported
func (*Callback) GetPluginGetter() plugingetter.PluginGetter {
	return nil
}

//RegisterDriver saves the libnetwork bridge driver instance
func (d *Callback) RegisterDriver(name string, driver driverapi.Driver, capability driverapi.Capability) error {
	d.driver = driver
	return nil
}
