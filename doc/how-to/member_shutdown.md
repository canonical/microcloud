---
myst:
  html_meta:
    description: Learn how to safely shut down a MicroCloud cluster member, including how to stop and migrate or live-migrate LXD instances running on it.
---

(howto-member-shutdown)=
# How to shut down a MicroCloud cluster member

This guide provides instructions to safely shut down a MicroCloud cluster member. This includes how to deal with LXD instances running on the cluster member.

(howto-member-shutdown-instances)=
## Stop or live-migrate all instances on the cluster member

To shut down a machine that is a MicroCloud cluster member, first ensure that it is not hosting any running LXD instances.

You can stop all instances on a cluster member using the command:

```bash
lxc stop --all
```

Alternatively, for instances that can be {ref}`live-migrated <lxd:live-migration>`, you can migrate them to another cluster member without stopping them. See: {ref}`lxd:howto-instances-migrate` for more information.

You can also temporarily migrate all instances on a machine to another cluster member by using cluster evacuation, then restore them to the original host after it is restarted. This method can live-migrate eligible instances; instances that cannot be live-migrated are automatically stopped and restarted. See: {ref}`lxd:cluster-evacuate` for more information.

(howto-member-shutdown-restart)=
## Shut down and restart

Once there are no running instances on the cluster member, you can safely shut it down, then restart it if desired. 

```{admonition} Services stop and restart order
:class: note
During the shutdown process of a MicroCloud cluster member, the LXD snap ensures that the LXD service stops _before_ the MicroCeph and MicroOVN services. At restart, the LXD service automatically starts _after_ the MicroCeph and MicroOVN services. This enforced order ensures that LXD does not run into issues due to unavailable storage or networking services.
```
