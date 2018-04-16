package utils

import (
	"crypto/md5"
	"encoding/hex"
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

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
