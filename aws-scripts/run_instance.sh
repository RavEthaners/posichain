#!/bin/bash -x
echo "Run Instances" >> tmplog

sudo sysctl net.core.somaxconn=1024
sudo sysctl net.core.netdev_max_backlog=65536;
sudo sysctl net.ipv4.tcp_tw_reuse=1;
sudo sysctl -w net.ipv4.tcp_rmem='65536 873800 1534217728';
sudo sysctl -w net.ipv4.tcp_wmem='65536 873800 1534217728';
sudo sysctl -w net.ipv4.tcp_mem='65536 873800 1534217728';

./kill_node.sh
MyHOME=/home/ec2-user
source ~/.bash_profile
export GOROOT=/usr/lib/golang
export GOPATH=$MyHOME/projects
export PATH=$PATH:$GOROOT/bin

# Get my IP
wget http://169.254.169.254/latest/meta-data/public-ipv4
ip=$(head -n 1 public-ipv4)
echo "Current IP is >>>"
echo $ip
echo ">>>>"

# Run soldier
cd $GOPATH/src/harmony-benchmark
node_port=3000
./bin/soldier -ip $ip -port $node_port&