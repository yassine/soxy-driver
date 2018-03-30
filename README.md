# soxy-driver
A docker networking driver that redirect all the container TCP traffic through a Proxy. 
The driver uses docker's core networking library [libnetwork](https://github.com/docker/libnetwork) and benefits thus from its stability.
With respect to TCP-tunneling, [redsocks](https://github.com/darkk/redsocks/) is used behind the scenes, supporting thus many tunneling strategies : socks4, socks5, http-connect, http-relay.

## Usage
The following example assumes tor is running on the host and exposes a socks5 proxy on port 9050:

1) Build the image
`docker build . -t yassine/soxy-driver`
2) Run the driver container
`docker run -d --net host --name soxy-driver yassine/soxy-driver`
3) Create a network based on the driver
`docker network create -d soxy-driver soxy_network -o "soxy.proxyaddress"="localhost" -o "soxy.proxyport"="9050"`
4) Test a container
`docker run --rm -it --net soxy_network uzyexe/curl -s https://check.torproject.org/api/ip`
5) Expected output
`{"IsTor":true,"IP":"%TOR_EXIT_NODE_IP%"}`

## Configuration options
Configuration options are passed when creating a given network (See example above). Available options are :

Option | Description | Default
--- | --- | ---
*soxy.proxyaddress* | The address of the proxy through which the traffic is redirected | localhost
*soxy.proxyport* | The proxy port | 9050 (tor)
*soxy.proxytype* | The proxy type | socks5 (available choices : socks4, socks5, http-connect, http-relay)
*soxy.proxyuser* | The proxy user if the proxy requires Authentication | none
*soxy.proxypassword* | The proxy user if the proxy requires Authentication | none
