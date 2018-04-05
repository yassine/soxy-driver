package driver

import (
	"github.com/docker/go-plugins-helpers/network"
	"testing"
)

func TestJoinInfoProxyInit(t *testing.T) {

}

func createMockJoinInfo() *JoinInfoProxy {
	proxy := &JoinInfoProxy{
		request:  mockJoinInfoRequest(),
		response: &network.JoinResponse{},
	}
	return proxy
}

func mockJoinInfoRequest() *network.JoinRequest {
	return &network.JoinRequest{
		NetworkID:  "NT0000",
		EndpointID: "EP0000",
		Options:    make(map[string]interface{}),
		SandboxKey: "SB0000",
	}
}
