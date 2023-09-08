# Copyright 2012-2021 Canonical Ltd.  This software is licensed under the
# GNU Affero General Public License version 3 (see the file LICENSE).

# README FIRST
# You need a reasonably powerful bare metal machine, 4 or more cores with 32 GB of RAM and 200GB of free disk space. Assumes a fresh install of Ubuntu server (20.04 or higher) on the machine.
# You need a bare metal machine is because nesting multiple layers of VMs will not work and/or have performance problems.

sudo snap install --channel=latest/stable lxd
sudo snap refresh --channel=latest/stable lxd
sudo snap install jq
sudo snap install maas
sudo snap install maas-test-db

# get local interface name (this assumes a single default route is present)
export INTERFACE=$(ip route | grep default | awk '!/wlan|wlp|wifi/ {print $5}')
export IP_ADDRESS=$(ip -4 addr show dev $INTERFACE | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
sudo sed -i 's/#net.ipv4.ip_forward=1/net.ipv4.ip_forward=1/' /etc/sysctl.conf
sudo sysctl -p
sudo iptables -t nat -A POSTROUTING -o $INTERFACE -j SNAT --to $IP_ADDRESS

# Persist NAT configuration
echo iptables-persistent iptables-persistent/autosave_v4 boolean true | sudo debconf-set-selections
echo iptables-persistent iptables-persistent/autosave_v6 boolean true | sudo debconf-set-selections
sudo apt-get install iptables-persistent -y

# LXD init
cat lxd.conf | sudo lxd init --preseed
# verify LXD network config
lxc network show lxdbr0
# Wait for LXD to be ready
lxd waitready
# Initialise MAAS
sudo maas init region+rack --database-uri maas-test-db:/// --maas-url http://${IP_ADDRESS}:5240/MAAS
# Sleeping for awhile to let MAAS do what it needs to do.
sleep 30
# Create MAAS admin and grab API key
sudo maas createadmin --username admin --password admin --email admin
export APIKEY=$(sudo maas apikey --username admin)
# MAAS admin login
maas login admin 'http://localhost:5240/MAAS/' $APIKEY
# Configure MAAS networking (set gateways, vlans, DHCP on etc). If you encounter errors
# here, it might be because MAAS hasn't finished initialising. You can try waiting a bit and rerunning.
export SUBNET=10.10.10.0/24
export FABRIC_ID=$(maas admin subnet read "$SUBNET" | jq -r ".vlan.fabric_id")
export VLAN_TAG=$(maas admin subnet read "$SUBNET" | jq -r ".vlan.vid")
export PRIMARY_RACK=$(maas admin rack-controllers read | jq -r ".[] | .system_id")
maas admin subnet update $SUBNET gateway_ip=10.10.10.1
maas admin ipranges create type=dynamic start_ip=10.10.10.200 end_ip=10.10.10.254
maas admin vlan update $FABRIC_ID $VLAN_TAG dhcp_on=True primary_rack=$PRIMARY_RACK
maas admin maas set-config name=upstream_dns value=8.8.8.8
# Add LXD as a VM host for MAAS and capture the VM_HOST_ID
export VM_HOST_ID=$(maas admin vm-hosts create  password=password  type=lxd power_address=https://${IP_ADDRESS}:8443 \
 project=maas | jq '.id')

# allow high CPU oversubscription so all VMs can use all cores
maas admin vm-host update $VM_HOST_ID cpu_over_commit_ratio=4

# create tags for MAAS
maas admin tags create name=juju-controller comment='This tag should to machines that will be used as juju controllers'
maas admin tags create name=metal comment='This tag should to machines that will be used as bare metal'

### creating VMs for Juju controller and our "bare metal"

# add a VM for the juju controller with minimal memory
maas admin vm-host compose $VM_HOST_ID cores=1 memory=2048 architecture="amd64/generic" storage="main:16(pool1)" hostname="juju-controller"
# get the system-id and tag the machine with "juju-controller"
export JUJU_SYSID=$(maas admin machines read | jq  '.[] | select(."hostname"=="juju-controller") | .["system_id"]' | tr -d '"')
maas admin tag update-nodes "juju-controller" add=$JUJU_SYSID

## Create 3 "bare metal" machines and tag them with "metal"
for ID in 1 2 3
do
    maas admin vm-host compose $VM_HOST_ID cores=4 memory=4096 architecture="amd64/generic" storage="main:10(pool1),ceph:20(pool1)" hostname="metal-${ID}"
	SYSID=$(maas admin machines read | jq -r --arg MACHINE "metal-${ID}" '.[] | select(."hostname"==$MACHINE) | .["system_id"]' | tr -d '"')
    maas admin tag update-nodes "metal" add=$SYSID
done


### Juju setup (note, this section requires manual intervention)
cd ~
sudo snap install juju --classic
sed -i "s/IP_ADDRESS/$IP_ADDRESS/" maas-cloud.yaml
juju add-cloud maas-cloud maas-cloud.yaml
juju add-credential maas-cloud
juju clouds
juju credentials

# Go on the MAAS UI at $IP_ADDRESS:5240/MAAS, login (username: admin, password: admin), and add Ubuntu 22.04/amd64 in the available images.
# Wait for MAAS to download the image and continue. This step is required for the following `juju bootstrap` command to complete.

# Bootstrap the maas-cloud - get a coffee
juju bootstrap maas-cloud --bootstrap-constraints "tags=juju-controller mem=2G"

juju add-model microcloud-maas maas-cloud

# check jujus view of machines
juju machines

# add machines to juju from the maas cloud
# it will grab the 3 we already created since they are in a "READY state"
for ID in 1 2 3
do
    juju add-machine
done

# take a look at machines list again, you should see 3 machines. Wait for all of them to be in a "started" state. It could take a while...
juju machines

# Create a model in juju
juju add-model microcloud

# Pack the charm and deploy it to our 3 machines
cd ../ && charmcraft pack && juju deploy ./microcloud_ubuntu-22.04-amd64.charm -n 3 --to 0,1,2

# You can add more machines to the model and scale up the application

# Let's add the 'metal-4' machine on MAAS and add it to the juju model
maas admin vm-host compose $VM_HOST_ID cores=4 memory=4096 architecture="amd64/generic" storage="main:10(pool1),ceph:20(pool1)" hostname="metal-4"
SYSID=$(maas admin machines read | jq -r --arg MACHINE "metal-4" '.[] | select(."hostname"==$MACHINE) | .["system_id"]' | tr -d '"')
maas admin tag update-nodes "metal" add=$SYSID

# Add the machine to juju once the machine is in a "ready" state
juju add-machine
juju machines

# Scale up the application to the new machine (the new machine should have the identifier '3' on JuJu)
juju add-unit microcloud -n 1 --to 3

# Scale down the application
juju remove-unit microcloud/3