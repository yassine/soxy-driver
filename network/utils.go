package network

import (
	"github.com/yassine/soxy-driver/utils"
	"os"
	"strings"
)

func iptablesSoxyChainName() string {
	if len(os.Getenv("DRIVER_NAMESPACE")) == 0 {
		return defaultChainName
	}
	parts := []string{strings.TrimSpace(os.Getenv("DRIVER_NAMESPACE")), defaultChainName}
	preResult := utils.GetMD5Hash(strings.Join(parts, "__"))[0:15]
	parts = []string{preResult, defaultChainName}
	return strings.Join(parts, "__")
}

