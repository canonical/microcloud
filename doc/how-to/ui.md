---
myst:
  html_meta:
    description: The MicroCloud UI provides a browser-based user interface to manage all aspects of your cluster, including instances, networks, and more.
---

(howto-ui)=
# How to access the MicroCloud UI

MicroCloud includes access to a browser-based graphical user interface. Through this UI, you can create and manage instances, networks, storage pools, images, permission groups, and more.

For example, the screenshot below displays the network configuration view in the UI:

```{figure} /images/ui_network.png
:alt: Network configuration view in the UI
```

For further views of the UI, see more [example screenshots](https://github.com/canonical/lxd-ui?tab=readme-ov-file#examples).

(howto-ui-use)=
## Using the UI

The MicroCloud UI is functionally the same interface as the {ref}`LXD UI <lxd:access-ui>`. For detailed UI usage information for specific functions, refer to the LXD documentation. Within any LXD how-to guide where the option to use the UI exists, you'll find a UI tab. For an example, see LXD's {ref}`lxd:network-configure` guide.

(howto-ui-access)=
## Access the UI

The UI is enabled when you initialize MicroCloud. To access it, enter the URL of any of the cluster members (including the port `:8443`) in your browser. For available URLs, run:

```bash
lxc cluster list
```

When logged into a cluster member, you can use `https://localhost:8443`.

The first time you use the UI, you will encounter a security warning. Follow the instructions in the LXD documentation to {ref}`set up secure access <lxd:access-ui-setup>`.
