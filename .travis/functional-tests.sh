#!/bin/sh

#########################################################################################
### This tests are meant to be executed by both travis and locally during development ###
#########################################################################################

# Setup remote iperf server used to test udp connectivity
IPERF_SERVER=${IPERF_SERVER:-"iperf.volia.net"}

docker pull uzyexe/curl
docker pull mlabbe/iperf3

# A network with default settings
docker network create -d soxy-driver soxy_network
# A network that prohibit UDP traffic
docker network create -d soxy-driver soxy_network_no_udp -o "soxy.blockUDP"="true"

###############################
# default settings test suite #
###############################

echo "TCP traffic should be tunneled through tor (using default socks proxy)"

docker run --rm --dns 8.8.8.8 -it --net soxy_network uzyexe/curl -s --retry-delay 3 --retry 10 https://check.torproject.org/api/ip  | jq '.IsTor' | grep true
ec=$?
if [ ! $ec -eq 0 ]
then
  echo "TCP traffic tunneling seems failing"
  echo "got exit status $ec"
  exit 1
fi

echo "UDP traffic should be allowed when a docker network is configured so"

docker run --rm -it --dns 8.8.8.8 --net soxy_network mlabbe/iperf3 --client ${IPERF_SERVER} --udp -J -k 1
ec=$?
if [ ! $ec -eq 0 ]
then
  # As this may be due to a the iperf server failure (e.g. server busy)
  # Script failure is avoided here
  echo "UDP traffic tunneling seems failing"
fi

###############################
###### no-udp test suite ######
###############################


echo "DNS should still be resolved by containers that use a network that prohibit UDP trafic"

docker run --rm --dns 8.8.8.8 -it --net soxy_network_no_udp uzyexe/curl -s --retry-delay 3 --retry 10 https://check.torproject.org/api/ip  | jq '.IsTor' | grep true
ec=$?
if [ ! $ec -eq 0 ]
then
  #DNS resolution should pass
  echo "DNS traffic tunneling failed when prohibiting UDP"
  echo "got exit status $ec"
  exit 1
fi

echo "UDP traffic should be blocked when a docker network is configured so"

docker run --rm -it --dns 8.8.8.8 --net soxy_network_no_udp mlabbe/iperf3 --client ${IPERF_SERVER} --udp -J -k 1
ec=$?
if [ $ec -eq 0 ]
then
  echo "UDP traffic prohibition failed"
  exit 1
fi


##################################
###### shutdown test suite  ######
##################################

docker network rm soxy_network
docker network rm soxy_network_no_udp
sleep 1
docker stop soxy-driver
sleep 3
docker rm soxy-driver

if [ -f "/run/docker/plugins/soxy-driver.sock" ]
then
  #socket file should have been removed
  echo "found /run/docker/plugins/soxy-driver.sock"
  exit 1
fi

##################################
###### namespace test suite ######
##################################

docker run -d -e DRIVER_NAMESPACE='testing' -v '/var/run/docker.sock':'/var/run/docker.sock' -v '/run/docker/plugins':'/run/docker/plugins' --net host --name namespaced-soxy-driver --privileged yassine-soxy-driver
docker network create -d testing__soxy-driver namespaced_driver_soxy_network
docker run --rm --dns 8.8.8.8 -it --net namespaced_driver_soxy_network uzyexe/curl -s --retry-delay 3 --retry 10 https://check.torproject.org/api/ip  | jq '.IsTor' | grep true
ec=$?
if [ ! $ec -eq 0 ]
then
  #DNS resolution should pass
  echo "got exit status $ec"
  exit 1
fi

docker network rm namespaced_driver_soxy_network
docker stop namespaced-soxy-driver
docker rm namespaced-soxy-driver

if [ -f "/run/docker/plugins/testing__soxy-driver.sock" ]
then
  #socket file should have been removed
  echo "found /run/docker/plugins/testing__soxy-driver.sock"
  exit 1
fi

echo ""
echo "############## TESTING ENDED ##############"
