#!/bin/sh

### This tests are meant to be executed by both travis and locally during development ###

#Blocking DNS traffic system wide
docker pull uzyexe/curl

sudo iptables -I OUTPUT -p udp --dport 53 -j DROP
sudo iptables -I INPUT -p udp --sport 53 -j DROP

dig www.google.com @8.8.8.8
ec=$?

if [ $ec -eq 0 ]
then
  #DNS resolution should have failed as dns is blocked
  echo "warning: resolution should have failed"
  echo "got exit status $ec"
  exit 1
fi

#DNS should still be resolved by containers that use a network based on the driver as those requests are tunneled
docker run --rm --dns 8.8.8.8 -it --net soxy_network uzyexe/curl -s --retry-delay 3 --retry 10 https://check.torproject.org/api/ip  | jq '.IsTor' | grep true

ec=$?
if [ ! $ec -eq 0 ]
then
  #DNS resolution should pass
  echo "got exit status $ec"
  exit 1
fi

#recover system wide
sudo iptables -D OUTPUT -p udp --dport 53 -j DROP
sudo iptables -D INPUT -p udp --sport 53 -j DROP

docker network rm soxy_network
docker stop soxy-driver
docker rm soxy-driver

if [ -f "/run/docker/plugins/soxy-driver.sock" ]
then
  #socket file should have been removed
  echo "found /run/docker/plugins/soxy-driver.sock"
  exit 1
fi

### Testing namespace capabilities
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
docker stop testing__soxy-driver
docker rm testing__soxy-driver

if [ -f "/run/docker/plugins/testing__soxy-driver.sock" ]
then
  #socket file should have been removed
  echo "found /run/docker/plugins/testing__soxy-driver.sock"
  exit 1
fi

echo ""
echo "############## TESTING ENDED ##############"
