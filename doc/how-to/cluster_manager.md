---
myst:
html_meta:
description: How-to guide for installing and using the MicroCloud Cluster Manager, a Kubernetes-based web application for viewing and managing multiple MicroCloud deployments.
---

(howto-cluster-manager)=
# How to manage multiple clusters with the Cluster Manager

This page provides instructions for installing the MicroCloud Cluster Manager and using it to view resource usage and availability across multiple clusters.

For detailed technical information, see {ref}`ref-cluster-manager-architecture`.

```{admonition} Cluster Manager versus the MicroCloud UI
:class: note

The MicroCloud Cluster Manager is a separate application from the {ref}`MicroCloud UI <howto-ui>`. The MicroCloud UI is used to manage a single cluster or single LXD deployment.
```

(howto-cluster-manager-requirements)=
## Requirements

The clusters that you intend to manage with the Cluster Manager must all use **MicroCloud 3.0/edge or higher**.

The Cluster Manager itself must be installed on a machine with the following setup:

- Juju 3.6 or higher
- A Kubernetes deployment controlled by Juju
- An OIDC client configured
- Two resolvable domain names or static IPs
  - Domain names are preferred
  - One must be accessible from the MicroCloud clusters
  - One must be acccessible by the Cluster Manager user

(howto-cluster-manager-requirements-oidc)=
### OIDC client configuration

You need a supported OpenID Connect (OIDC) account with a client application configured.

The {ref}`LXD documentation on OIDC <lxd:howto-oidc>` provides instructions to configure compatible OIDC providers for authenticating with the LXD UI and CLI.

Use these instructions to configure the OIDC client on the provider side for use with the Cluster Manager, with these modifications:

- When creating the client application, use port `443` instead of `8443` as the listen port for the callback URL. `8443` is the default listen port for the LXD server; `443` is the default for the Cluster Manager.

- When shown how to obtain certain values for adding via `lxc config`, note down the values only. Do not use `lxc config` to set them. You'll need the values that are used for the following config values in LXD:

- `oidc.issuer`
- `oidc.client`
- `oidc.audience`  (Auth0 only)
- `oidc.client.secret` (Keycloak only)

You will use these values later for Juju configuration.

Once you have configured the OIDC client on the provider side and obtained these values, continue with the Cluster Manager installation steps below.

(howto-cluster-manager-install)=
## Install the Cluster Manager model and set up its charms

First, add a new `cluster-manager` Juju [model](https://documentation.ubuntu.com/juju/3.6/howto/manage-models/):

```bash
juju add-model cluster-manager
```

Deploy the [PostgreSQL K8s charm](https://charmhub.io/postgresql-k8s), the [Traefik K8s charm](https://charmhub.io/traefik-k8s), and the [MicroCloud Cluster Manager K8s charm](https://charmhub.io/microcloud-cluster-manager-k8s):

```bash
juju deploy postgresql-k8s --channel 14/stable --trust
juju deploy traefik-k8s --trust
juju deploy microcloud-cluster-manager-k8s --channel edge --trust
```

The `--trust` flag grants the deployed charm permission to access cloud or cluster credentials and perform privileged operations.

A certificate charm must also be deployed to manage TLS/SSL certificates for secure communication within the K8s cluster. Any charm that implements both the [certificates](https://charmhub.io/microcloud-cluster-manager-k8s/integrations#certificates) and [send-ca-cert](https://charmhub.io/microcloud-cluster-manager-k8s/integrations#send-ca-cert) interfaces can be used, such as the [self-signed-certificates charm](https://charmhub.io/self-signed-certificates).

Deploy your chosen certificate charm:

```bash
juju deploy <certificate charm> --trust
```

Example using the [self-signed certificates charm](https://charmhub.io/self-signed-certificates) charm:

```bash
juju deploy self-signed-certificates --trust
```

Next, use `juju integrate` to create virtual connections between the charmed applications you deployed, so that they can communicate:

```bash
juju integrate postgresql-k8s:database microcloud-cluster-manager-k8s
juju integrate traefik-k8s:traefik-route microcloud-cluster-manager-k8s
```

Also integrate the certificate charm you used with the MicroCloud Cluster Manager K8s charm.

```bash
juju integrate <certificate charm>:certificates microcloud-cluster-manager-k8s
juju integrate <certificate charm>:send-ca-cert microcloud-cluster-manager-k8s
```

For example, if you used the self-signed certificates charm, run:

```bash
juju integrate self-signed-certificates:certificates microcloud-cluster-manager-k8s
juju integrate self-signed-certificates:send-ca-cert microcloud-cluster-manager-k8s
```

(howto-cluster-manager-oidc-access)=
## Set up OIDC access for the Cluster Manager

From the {ref}`howto-cluster-manager-requirements-oidc` section above, you should have obtained values for the following: the OIDC issuer, client ID, client secret, and audience.

Use the following syntax to add them to the Cluster Manager configuration in Juju:

```bash
juju config microcloud-cluster-manager-k8s oidc-issuer=<your OIDC issuer)
juju config microcloud-cluster-manager-k8s oidc-client-id=<your OIDC client ID>
juju config microcloud-cluster-manager-k8s oidc-audience=<your OIDC audience> # for Auth0 only
juju config microcloud-cluster-manager-k8s oidc-client-secret=<your OIDC client secret> # for Keycloak only
```

Configure a domain for the {ref}`ref-cluster-manager-architecture-management`, and another for the {ref}`ref-cluster-manager-architecture-connector`. You can also use IP addresses, but using domains is recommended.

The domain or IP address used for the `management-api-domain` must be able to be accessed by the Cluster Manager user. The Cluster Connector must be able to be accessed by the MicroCloud clusters.

```bash
juju config microcloud-cluster-manager-k8s management-api-domain=<domain or IP for the Management API>
juju config microcloud-cluster-manager-k8s cluster-connector-domain=<domain or IP for the Cluster Connector>
```

Example:

```bash
juju config microcloud-cluster-manager-k8s management-api-domain=management-api.example.com
juju config microcloud-cluster-manager-k8s cluster-connector-domain=cluster-connector.example.com
```

Set the `external_hostname` for your Traefik controller to the same value as the ``management-api-domain` used above:

```bash
juju config traefik-k8s external_hostname=<management-api-domain>
```

Example:

```bash
juju config traefik-k8s external_hostname=management-api.example.com
```

Now you can access the Cluster Manager's web UI in your browser through that address.

The web UI should prompt you to log in. Use the username and password for the OIDC account with which you {ref}`configured an OIDC client <howto-cluster-manager-requirements-oidc>` earlier.

(howto-cluster-manager-enroll)=
## Enroll your first cluster

Use the web UI to enroll your first cluster. Alternatively, use the `enroll-cluster` CLI command to create a join token for your first cluster, providing a cluster name of your choice:

```bash
juju run microcloud-cluster-manager-k8s/0 enroll-cluster cluster=<cluster-name>
```

Example:

```bash
juju run microcloud-cluster-manager-k8s/0 enroll-cluster cluster=microcloud-01
```

Once the cluster is enrolled, you can explore its details in the Cluster Manager.

(howto-cluster-manager-cos)=
## Extend with observability

You can extend Cluster Manager with the [Canonical Observability Stack (COS)](https://canonical.com/observability) for Grafana and Prometheus integration.

Add the `cos` model and deploy the [COS Lite charm](https://charmhub.io/cos-lite):

```bash
juju add-model cos
juju deploy cos-lite --trust
```

The following commands expose application interfaces (endpoints) in the `cos` model for cross-model relations. For the `prometheus` application, we expose its `receive-remote-write` interface available so that other models (specifically, the `cluster-manager` model) can connect and send remote write metrics. For the `grafana` application, we expose the `grafana-dashboard` and `grafana-db` interfaces so that the other models can connect and use Grafana dashboards and its database. We also expose the `grafana-metadata` interface for sharing metadata with other models.

Run:

```bash
juju offer prometheus:receive-remote-write
juju offer grafana:grafana-dashboard grafana-db
juju offer grafana:grafana-metadata
```

Switch to the Cluster Manager controller and model:

```bash
juju switch cluster-manager
```

Enable relations between the MicroCloud Cluster Manager K8s application and the interfaces we exposed in the COS model's applications.

```bash
juju integrate microcloud-cluster-manager-k8s:send-remote-write admin/cos.prometheus
juju integrate microcloud-cluster-manager-k8s:grafana-dashboard admin/cos.grafana-db
juju integrate microcloud-cluster-manager-k8s:grafana-metadata admin/cos.grafana
```

This makes a LXD dashboard available in Grafana, and Cluster Manager now starts forwarding metrics to COS whenever it receives a cluster {ref}`heartbeat <ref-cluster-manager-architecture-connector-heartbeats>`.

To access Grafana, fetch the admin password:

```bash
juju run --model cos grafana/leader get-admin-password
```

Once you have completed this setup, a button on the cluster details page of the Cluster Manager web UI provides a deep-link into the Grafana dashboard.

## Related topics

Reference:

- {ref}`ref-cluster-manager-architecture`
