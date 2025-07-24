(howto-member-add)=
# How to add a cluster member to an existing MicroCloud

A MicroCloud cluster can consist of up to fifty cluster members. These can be added during {ref}`initialization <howto-initialize>` as well as at any time afterward, up to this limit.

## Interactive configuration

To add a cluster member to a MicroCloud, run the {command}`microcloud add` command on one of the existing cluster members:

```bash
sudo microcloud add
```

On the machine you want to add to the cluster, run the {command}`microcloud join` command:

```bash
sudo microcloud join
```

Answer the prompts on both sides to add the cluster member.

## Non-interactive configuration

To automate adding a cluster member, provide a preseed configuration in YAML format to the {command}`microcloud preseed` command:

```bash
cat <preseed_file> | microcloud preseed
```

In the list of systems, include only the new machine and set either `initiator` or `initiator_address`, which can point to any machine
that is already part of the MicroCloud.

Distribute and run the same preseed configuration on both the machine being added, and the cluster member used for the `initiator` or `initiator_address`.

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

Run the {command}`microcloud preseed` command on `micro01` and `micro04` to add the new cluster member.
