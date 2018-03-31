# soxy-driver
A docker networking driver that tunnels all the container TCP traffic through a Proxy. 
The driver uses docker's core networking library [libnetwork](https://github.com/docker/libnetwork) and benefits thus from its stability.

With respect to TCP-tunneling, [redsocks](https://github.com/darkk/redsocks/) is used behind the scenes, supporting therefore many proxy tunneling strategies : socks4, socks5, http-connect, http-relay.

## Usage
The following example uses the driver embedded tor proxy:

1) Build the image
`docker build . -t yassine/soxy-driver`
2) Run the driver container
`docker run -d -v '/var/run/docker.sock':'/var/run/docker.sock' -v '/run/docker/plugins':'/run/docker/plugins' --net host --name soxy-driver --privileged yassine/soxy-driver`
3) Create a network based on the driver
`docker network create -d soxy-driver soxy_network`

> Note: If you want to test against another proxy than the embedded tor-based one, you can pass the proxy params using
the `-o` option. For example : `docker network create -d soxy-driver soxy_network -o "soxy.proxyaddress"="%PROXY_HOST%" -o "soxy.proxyport"="%PROXY_PORT%"`, see the next section for all available
configuration options.

You can now create a container that uses the network formerly created and test the tunneling:
 
`docker run --rm -it --net soxy_network uzyexe/curl -s https://check.torproject.org/api/ip`

Output : `{"IsTor":true,"IP":"%SOME_TOR_EXIT_NODE_IP_HERE%"}`

## Configuration options
Configuration options are passed when creating a given network (See example above). Available options are :

Option | Description | Default
--- | --- | ---
*soxy.proxyaddress* | The address of the proxy through which the traffic is redirected | localhost
*soxy.proxyport* | The proxy port | A random port that maps to the embedded tor instance socks port
*soxy.proxytype* | The proxy type | socks5 (available choices : socks4, socks5, http-connect, http-relay)
*soxy.proxyuser* | The proxy user if the proxy requires Authentication | none
*soxy.proxypassword* | The proxy user if the proxy requires Authentication | none

Configuration params maps to a given network only, and thus must be passed when creating a network `docker network create`
