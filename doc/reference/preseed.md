(ref-preseed)=
# Preseed configuration options

MicroCloud preseed allows the unattended (non-interactive) deployment of a cluster using a pre-configured file.
See below for detailed descriptions of the main building blocks of the file, followed by a {ref}`full configuration example <ref-preseed-full-configuration-example>` file.

(ref-preseed-filters)=
## Storage disk filters

Explicitly setting the storage disks per system under `systems.[*].storage` is optional.
Use filters if the exact disk paths are unknown when crafting the preseed file.
This also makes the preseed file generic enough to be usable across various MicroCloud deployments.

Filters allow MicroCloud to make a selection from a list of all disks available on the systems.
These filters correspond to the YAML field names of the disk resources returned from LXD's `/1.0/resources` endpoint.

The following table lists all of the available filters:

```{table}
:align: left

| Filter             | Example                                     |
| ------------------ | ------------------------------------------- |
| `id`               | `nvme0n1`                                   |
| `device`           | `259:0`                                     |
| `model`            | `<Vendor>`                                  |
| `type`             | `nvme`                                      |
| `read_only`        | `false`                                     |
| `mounted`          | `false`                                     |
| `size`             | `1024209543168` (size of the disk in bytes) |
| `removable`        | `false`                                     |
| `wwn`              | `eui.00xxxxxxxxxxxxxx`                      |
| `numa_node`        | `0`                                         |
| `device_path`      | `pci-0000:04:00.0-nvme-1`                   |
| `block_size`       | `512`                                       |
| `firmware_version` | `4L2XXXXX`                                  |
| `rpm`              | `0`                                         |
| `serial`           | `S7XKXXXXXXXXXX`                            |
| `device_id`        | `nvme-eui.00xxxxxxxxxxxxxx`                 |
| `pci_address`      | `0000:04:00.0`                              |
| `used_by`          | `bcache`                                    |
```

When using the `size` filter, its value can be compared against a user-defined number using byte suffixes in either units of 1000 or 1024:

`B`, `kB`, `MB`, `GB`, `TB`, `EB`, `KiB`, `MiB`, `GiB`, `TiB`, `PiB`, `EiB`

### Filter operands

All filters can use the following operands to compare against defined values:

`&&`, `||`, `<`, `>`, `<=`, `>=`, `==`, `!=`, `!`

Furthermore the following restrictions apply:

* Filters are checked in order of appearance
* String values must not be in quotes unless the string contains a space
* Single quotes are fine, but double quotes must be escaped


Multiple filters can be added to a single section:

```yaml
storage:
  ceph:
    - find: <filters>
    - find: <filters>
```

### Limit filtered disks

In addition to finding disks by filter, the minimum and maximum number of disks can also be specified.
For this, the `find_min` and `find_max` settings can be added to the relevant section:

```yaml
storage:
  ceph:
    - find: <filters>
      find_min: 1
      find_max: 2
```

The example above will make sure that the filters select at least one, but not more than two, disks for remote (Ceph) storage.

```{note}
For local storage there can only ever be one disk per system.
If the filters return more than one disk, only one of them will be used.

For remote storage the filters apply for all disks across all systems.
```

(ref-preseed-full-configuration-example)=
## Full configuration example

The preseed YAML file must use the following syntax:

```{literalinclude} preseed.yaml
:language: YAML
:emphasize-lines: 1-4,7-10,13-14,17-19,22,25-27,30-35,63-66,72,79-88
```
