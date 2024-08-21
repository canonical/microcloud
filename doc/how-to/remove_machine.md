(howto-remove)=
# How to remove a machine

If you want to remove a machine from the MicroCloud cluster after the initialisation, use the {command}`microcloud remove` command:

    sudo microcloud remove <name>

Before removing the machine, ensure that there are no LXD instances or storage volumes located on the machine, and that there are no MicroCeph OSDs located on the machine.

See {ref}`how to remove instances <lxd:instances-manage-delete>` in the LXD documentation.
See {doc}`how to remove OSDs <microceph:how-to/remove-disk>` in the MicroCeph documentation.

If the machine is no longer reachable over the network, you can also add the `--force` flag to bypass removal restrictions and skip attempting to clean up the machine. Note that MicroCeph requires `--force` to be used if the remaining cluster size will be less than 3.

```{caution}
Removing a cluster member with `--force` will not attempt to perform any clean-up of the removed machine. All services will need to be fully re-installed before they can be re-initialised. Resources allocated to the MicroCloud like disks and network interfaces may need to be re-initialised as well.
```
