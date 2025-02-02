(howto-shutdown)=
# How to shut down a machine

## Stop all instances on the cluster member

Before you can shut down a machine that is a MicroCloud cluster member, first stop all LXD instances hosted on it:

```
lxc stop --all
```

## Enforce services shutdown and restart order

During the shutdown process of a MicroCloud cluster member, the LXD service must stop _before_ the MicroCeph and MicroOVN services. At restart, the LXD service must start _after_ MicroCeph and MicroOVN. This order ensures that LXD does not run into issues due to unavailable storage or networking services.

To enforce this shutdown and restart order, create a configuration file in each cluster member's `/etc/systemd/system/snap.lxd.daemon.service.d` directory to override the behaviour of `snap.lxd.daemon.service`. To simplify creating the directory and configuration file, you can copy and paste the following commands into each cluster member:

```
# Create the directory if it doesn't exist
sudo mkdir -p /etc/systemd/system/snap.lxd.daemon.service.d

# Create the configuration file
cat << EOF | sudo tee /etc/systemd/system/snap.lxd.daemon.service.d/lxd-shutdown.conf
# Makes sure the LXD daemon stops before Ceph/OVN and restarts after Ceph/OVN
[Unit]
After=snap.microceph.daemon.service
After=snap.microovn.daemon.service
EOF

# Reload systemd daemon
sudo systemctl daemon-reload
```

You only need to perform this step once for each cluster member. Afterwards, the `snap.lxd.daemon.service` respects this configuration at every shutdown and restart.

### Shut down

Once you have completed the steps above, you can safely shut down and restart the machine as normal. 