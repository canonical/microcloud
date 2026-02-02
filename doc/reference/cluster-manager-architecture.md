---
myst:
  html_meta:
    description: Reference architecture for MicroCloud Cluster Manager, a Kubernetes-based web application for viewing and managing multiple MicroCloud deployments.
---

(ref-cluster-manager-architecture)=
# Architecture of the MicroCloud Cluster Manager

The MicroCloud Cluster Manager is a centralized tool that provides an overview of MicroCloud deployments. In its initial implementation, it provides an overview of resource usage and availability for all clusters. Future implementations will include centralized cluster management capabilities.

Cluster Manager stores the data from registered clusters in Postgres and Prometheus databases. This data can be displayed in the Cluster Manager UI, which can be extended to link to Grafana dashboards for each MicroCloud.

(ref-cluster-manager-architecture-overview)=
## Architecture overview

Cluster Manager is a distributed web application that runs inside a Kubernetes cluster to achieve high availability. The diagram below illustrates its system architecture:

```{figure} ../images/cluster_manager_architecture.svg
   :alt: A diagram of Cluster Manager architecture
   :align: center
```

Inside the Kubernetes cluster are the following system components:

DNS
: The Domain Name Server (DNS) must be set up by the user to resolve their domain names to a static external IP. That static external IP acts as the gateway to route user traffic to the appropriate Kubernetes load balancers.

TCP load balancer
: A TCP load balancer (using a [Traefik](https://traefik.io/) instance) distributes traffic to the Management API and Cluster Connector without terminating TLS. TLS termination is handled directly within those two components. This approach is particularly crucial for the Cluster Connector, which relies on mutual TLS (mTLS) authentication for secure communication.

Postgres charm
: A PostgreSQL database deployed within the Kubernetes cluster using a charm. It provides persistent storage for system data. The Management API and Cluster Connector both communicate with the Postgres database for CRUD operations.

{ref}`ref-cluster-manager-architecture-management`
: Responsible for serving the static UI assets, exposing API endpoints for the UI to communicate with the Cluster Manager backend. Requests from the UI are authenticated using OpenID Connect (OIDC). Deployed along with the Cluster Connector inside one or more containers.

{ref}`ref-cluster-manager-architecture-connector`
: Responsible for handling requests from MicroCloud clusters, authenticated using mutual TLS (mTLS). Deployed along with the Management API inside a container inside one or more containers.

Certificate charm
: The certificate charm manages TLS/SSL certificates for secure communication within the Kubernetes cluster. It stores the Kubernetes secrets managed by the Juju configuration layer. The certificates are used by the Management API and the Cluster Connector for HTTPS encryption

: Juju configuration layer
Canonical's [Juju application orchestration engine](https://canonical.com/juju) is used to manage Kubernetes configuration and secrets.  

Canonical Observability Stack
: Optional. The Cluster Connector connects to the [Canonical Observability Stack](https://documentation.ubuntu.com/observability) (COS) through Grafana, reading from the Prometheus database. COS has its own load balancer that is distinct from the Cluster Manager load balancer. 

(ref-cluster-manager-architecture-management)=
## Management API

The management API handles local operations in Cluster Manager, including:

- Listing active MicroCloud clusters
- Creating cluster join tokens
- Approving join requests from clusters
- Serving the UI's static assets

(ref-cluster-manager-architecture-management-ingress)=
### Management API ingress

The Management API configures and runs an HTTPS server to make API endpoints available. Traffic to the server passes through a TCP load balancer.

(ref-cluster-manager-architecture-management-oidc)=
### OIDC authentication

The Management API is secured using OIDC authentication, using the [`microcloud-cluster-manager-k8s`](https://charmhub.io/microcloud-cluster-manager-k8s) charm configurations. The charm handles providing OIDC information to the Kubernetes cluster and its `configMap`.

The OIDC flow is similar to the {ref}`approach implemented in LXD <lxd:authentication-openid>`:
- A user initiates the login flow from the UI. This makes a request to the `/oidc/login` endpoint, which redirects the user to the identity provider's authentication screen. At this stage, a callback endpoint (`*/oidc/callback`) is set in the redirect request.
- The user then enters their credentials to authenticate with the identity provider.
Upon successful authentication, the identity provider sends a request to the callback endpoint `*/oidc/callback` set in step 1.
- The request includes an authorization code. The callback endpoint uses this code to initiate the token exchange process with the identity provider and acquire the ID, access, and refresh tokens for the authenticated user.
- These tokens are set in the session cookie and the user is redirected to the base route of the UI.
- Subsequent requests use the session cookie to validate authentication.

(ref-cluster-manager-architecture-management-registering)=
### Registering clusters

To register a MicroCloud cluster with a Cluster Manager, the user first creates a remote cluster join token in the Manager. This token is base64-encoded and includes the following information:

| Key         | Description |
| ------------| ----------- |
| secret      | A secret to be used by a MicroCloud cluster for creating a [HMAC](https://en.wikipedia.org/wiki/HMAC) signature for the join request.      |
| expires_at  | Expiry date for the remote cluster join token. |
| address     | The address at which the MicroCloud Cluster Manager is reachable. This address can be a domain name or external static IP. |
| server_name | This is unique and stored for reference purposes in Cluster Manager to map which cluster the token belongs to. |
| fingerprint | The public key from the Cluster Connector certificate (secret in Kubernetes cluster). Used to establish mTLS between Cluster Manager and the cluster. |

On a member of the registering cluster, the token is used in the command `microcloud cluster-manager join <token>`. The join request is sent to the Cluster Manager for approval. The request payload includes the registering cluster's name and cluster certificate.

The registering cluster begins sending periodic heartbeats along with status updates to the Cluster Manager. These heartbeats and updates begin as soon as a join request response is received from the Manager.

This is due to the unidirectional nature of communication initialization between clusters and the Cluster Manager. The Cluster Manager can only receive and respond to data from the cluster; it cannot initiate a message to tell the cluster when the join request is approved. For details about this, see {ref}`ref-cluster-manager-architecture-connector-ingress`.

Once the Cluster Manager receives a join request, it tries to match the cluster name in the payload to an entry in its `remote_cluster_tokens` table. If it finds a match, it uses the corresponding token secret stored in that table to verify the join request HMAC signature. The validity of the remote cluster join token is also checked against its expiry date.

If a valid match is found, the matched token is removed and the cluster is registered with the following information:

- `name` - extracted from the token
- `cluster_certificate` - received from the join request from the registering cluster. This is the MicroCloud cluster certificate for establishing mutual TLS authentication with the Cluster Manager.

A corresponding entry in the remote_clusters table is created, adding the following information:

- `status` - `ACTIVE`

Once a cluster has been successfully registered, Cluster Manager begins storing status update data sent by the cluster.

(ref-cluster-manager-architecture-management-ui)=
### UI

The Management API serves the UI's static assets. Users can use the UI to access information about registered clusters, as well as create or view remote cluster join tokens.

The UI also serves high-level metrics insights and warnings, including aggregated instance counts and status distributions (such as started vs. stopped). Detailed per-instance metrics can be viewed with [Grafana](https://grafana.com/) dashboards if the Cluster Manager is extended to use the [Canonical Observability Stack (COS)](https://canonical.com/observability).

(ref-cluster-manager-architecture-connector)=
## Cluster Connector

The Cluster Connector handles operations between MicroCloud clusters and the Cluster Manager.

(ref-cluster-manager-architecture-connector-ingress)=
### Cluster Connector ingress

We expect each cluster to be able to reach the Cluster Manager. However, we do not expect the Manager to be able to reach each remote cluster directly. This is because clusters might not be exposed on an internet-facing IP, or they might run behind a firewall or NAT. Therefore, operations consist of ingress traffic only.

All endpoints are rate limited to avoid overwhelming the Cluster Manager.

(ref-cluster-manager-architecture-connector-mtls)=
### mTLS authentication

During the initial join request of a cluster, each cluster presents a certificate to the Cluster Manager. The Manager uses this certificate to authenticate all subsequent requests from the cluster, using mTLS. For efficiency considerations, these certificates are cached after the first authenticated request.

Due to the mTLS requirement, the TCP load balancer passes through TLS traffic and the Cluster Connector terminates TLS itself.

(ref-cluster-manager-architecture-connector-heartbeats)=
### Heartbeats

The `db-leader` of each connected MicroCloud cluster sends periodic heartbeats to the Cluster Manager, along with data about resource usage and availability. A heartbeat update includes the following information:

- Cluster level details including:
  - Number of cluster-wide instances and distribution of instance status (such as how many instances are stopped or started)
  - Number of cluster members and distribution of member status (number of members online, number of members in error status, and so on)
  - CPU, memory, and disk utilization for each cluster, as aggregated totals across all cluster members
- MicroCloud certificate within the request context for mTLS authentication. The certificate fingerprint is used to look up the cluster ID in a cache for updating cluster details.

For each heartbeat it receives, the Cluster Manager performs the following tasks:

- Match status update request by certificate fingerprint; check that the cluster exists and is marked as active.
- mTLS authentication check against the matched certificate
- Store and overwrite data in the `remote_cluster_details` table
