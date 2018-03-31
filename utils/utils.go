package utils

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
)

//LogAndThrowError return an error given an error message
func LogAndThrowError(message string, params ...interface{}) error {
	formattedMessage := fmt.Sprintf(message, params)
	logrus.Errorf(formattedMessage)
	return errors.New(formattedMessage)
}

//FindAvailablePort finds an available TCP port
func FindAvailablePort() int64 {
	addr, _ := net.ResolveTCPAddr("tcp", "localhost:0")
	l, _ := net.ListenTCP("tcp", addr)
	port := int64(l.Addr().(*net.TCPAddr).Port)
	l.Close()
	return port
}
