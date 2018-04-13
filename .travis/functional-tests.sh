#!/bin/sh

#Blocking DNS traffic system wide

sudo iptables -I OUTPUT -p udp --dport 53 -j DROP
sudo iptables -I INPUT -p udp --sport 53 -j DROP

dig www.google.com @ 8.8.8.8
ec=$?

if [ $ec -eq 0 ]
then
  #DNS resolution should have failed as dns is blocked
  echo "warning: resolution should have failed"
  echo "got exit status $ec"
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
