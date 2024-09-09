(howto-add)=
# How to add a machine
## Interactive configuration

If you want to add a machine to the MicroCloud cluster after the initialisation, use the {command}`microcloud add` command:

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
:emphasize-lines: 1-3,6-8,11-12,19,29-33
```
