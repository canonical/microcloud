(howto-add)=
# How to add a machine
## Interactive configuration

If you want to add a machine to the MicroCloud cluster after the initialization, use the {command}`microcloud add` command:

    sudo microcloud add

On the new machine use the {command}`microcloud join` command:

    sudo microcloud join

Answer the prompts on both sides to add the machine.
You can also add the `--wipe` flag to automatically wipe any disks you add to the cluster.

## Non-interactive configuration

If you want to automatically add a machine, you can provide a preseed configuration in YAML format to the {command}`microcloud preseed` command:

    cat <preseed_file> | microcloud preseed

In the list of systems include only the new machine and set either `initiator` or `initiator_address` which can point to any machine
that is already part of the MicroCloud.

Make sure to distribute and run the same preseed configuration on the new and existing system configured using either `initiator` or `initiator_address`.

The preseed YAML file must use the following syntax:

```{literalinclude} preseed.yaml
:language: YAML
:emphasize-lines: 1-4,7-10,13-14,17-19,22,25-27,30-35,63-66,72,79-87
```

### Minimal preseed using multicast discovery

You can use the following minimal preseed file to add another machine to an existing MicroCloud.
In this case `micro01` takes over the role of the initiator.
Multicast discovery is used to find the existing MicroCloud on the network:

The disk `/dev/sdb` will be used for the machine's local storage pool.
The already existing remote storage pool will be extended with `/dev/sdc`:

```yaml
lookup_subnet: 10.0.0.0/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_uplink_interface: eth1
  storage:
    local:
      path: /dev/sdb
    ceph:
      - path: /dev/sdc
```

Run the {command}`microcloud preseed` command on `micro01` and `micro04` to add the additional machine.
