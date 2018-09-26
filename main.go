package main

import (
	"github.com/docker/go-plugins-helpers/network"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"github.com/yassine/soxy-driver/driver"
	"github.com/yassine/soxy-driver/utils"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	//DriverName The name of the driver (used when creating a network for example)
	DriverName = "soxy-driver"
	//DockerSocket Docker client hook
	DockerSocket = "unix:///var/run/docker.sock"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	client, err := docker.NewClient(DockerSocket)
	utils.LogIfNotNull(err)
	soxyDriver := driver.New()
	networks, err := client.ListNetworks()

	namespace := os.Getenv("DRIVER_NAMESPACE")
	driverName := ""

	if len(namespace) == 0 {
		driverName = DriverName
	} else {
		parts := []string{namespace, DriverName}
		driverName = strings.Join(parts, "__")
	}

	if err != nil {
		panic(err)
	}

	var recoveredNetworks []docker.Network
	for _, dockerNetwork := range networks {
		if dockerNetwork.Driver == driverName {
			logrus.Debug(dockerNetwork.Driver, " ", dockerNetwork.Driver == driverName)
			recoveredNetworks = append(recoveredNetworks, dockerNetwork)
		}
	}
	soxyDriver.Recover(recoveredNetworks)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range c {
			if sig == syscall.SIGTERM {
				//Shutdown the driver
				soxyDriver.ShutDown()
			}
			os.Remove("/run/docker/plugins/" + driverName + ".sock")
			os.Exit(0)
		}
	}()

	h := network.NewHandler(&soxyDriver)
	serveError := h.ServeUnix(driverName, 0)
	if serveError != nil {
		logrus.Error(serveError)
	}
}
