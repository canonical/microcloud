(howto-charm-metrics)=
# How to configure metrics with the MicroCloud charm

MicroCloud ships a [Juju](https://canonical.com/juju) [charm](https://charmhub.io/microcloud) to manage observability for a MicroCloud cluster. The charm does not deploy or bootstrap MicroCloud itself. Instead it attaches to an existing cluster and collects metrics and logs from LXD, MicroCeph, and MicroOVN on each member.

This guide shows how to add your MicroCloud machines to Juju, deploy the charm, and set up relations to the Canonical Observability Stack (COS) so that you can collect and visualize metrics and logs.

## Prerequisites

- A bootstrapped MicroCloud cluster (see {ref}`howto-initialize`).
- A [Juju client](https://canonical.com/juju/docs/juju-cli/3.6/howto/manage-juju/), with a controller and model you can deploy into. See the Juju documentation for the full list of [supported clouds](https://canonical.com/juju/docs/juju-cli/3.6/reference/cloud/list-of-supported-clouds/).
- SSH access from the Juju client to each MicroCloud machine, to add them as [manual machines](https://canonical.com/juju/docs/juju-cli/3.6/reference/juju-cli/list-of-juju-cli-commands/add-machine/#command-juju-add-machine).
- A [Canonical Observability Stack (COS)](https://charmhub.io/topics/canonical-observability-stack) deployment (or access to one through [cross-model relations](https://canonical.com/juju/docs/juju-cli/3.6/howto/manage-relations/#add-a-cross-model-relation)) if you want to visualize the collected metrics and logs in Grafana.

## Add the MicroCloud cluster members to Juju

For every MicroCloud cluster member, add a manual machine in your Juju model:

    juju add-machine ssh:<user>@<host>

Repeat this for each MicroCloud cluster member. Note the machine IDs assigned by Juju (shown in `juju status`), as you will need them in the next step.

## Deploy the charm

Run this command to deploy one unit of the `microcloud` charm to each MicroCloud machine (provide the total number of units and the machine IDs):

    juju deploy microcloud --channel 3/stable --num-units <N> --to <machine-ids>

For example, to deploy the charm across three machines with IDs `0`, `1`, and `2`:

    juju deploy microcloud --channel 3/stable --num-units 3 --to 0,1,2

Check the deployment with:

    juju status

Each unit should become `active`. If a unit instead reports an error, check that it is actually a member of the MicroCloud cluster (see {ref}`howto-members-manage`).

## Configure metrics collection

The charm exposes configuration options for metrics collection, each with a default value. To customize your configuration, set a new value with `juju config microcloud <key>=<value>`:

| Option | Default | Description |
| --- | --- | --- |
| `scrape-interval` | `30s` | Scrape interval used by the collector for all scrape jobs. |
| `ceph-mgr-prometheus-port` | `9283` | Port on which the Ceph `mgr` Prometheus module binds on loopback. |
| `ceph-rbd-stats-pools` | `lxd_remote` | Comma-separated list of RBD pools to collect per-image I/O statistics for. An empty string disables per-image RBD stats. |
| `ceph-enable-perf-metrics` | `true` | Include Ceph performance counters in the Prometheus `mgr` module output. |
| `ovn-exporter-channel` | `latest/edge` | Snap Store channel for the `ovn-exporter` snap. |
| `lxd-metrics-listen-port` | `8444` | Port on which LXD's metrics listener binds on loopback. |

For example, to change the scrape interval:

    juju config microcloud scrape-interval=1m

## Set up relations to COS

The charm provides a `cos-agent` endpoint for metrics and a `logging` endpoint for logs. Set up a [Juju relation](https://canonical.com/juju/docs/juju-cli/3.6/reference/relation/#relation) to an [`opentelemetry-collector`](https://charmhub.io/opentelemetry-collector) deployment, which forwards the data to Prometheus, Loki, and Grafana in your COS model.

If COS runs in a separate model (the common case for a dedicated observability stack), [offer](https://canonical.com/juju/docs/juju-cli/3.6/howto/manage-relations/#add-a-cross-model-relation) the Prometheus, Loki, and Grafana endpoints from the COS model:

    juju offer prometheus:receive-remote-write
    juju offer loki:logging
    juju offer grafana:grafana-dashboard

Then switch to the MicroCloud model and consume the offers:

    juju consume <cos-controller>:<user>/<cos-model>.prometheus
    juju consume <cos-controller>:<user>/<cos-model>.loki
    juju consume <cos-controller>:<user>/<cos-model>.grafana

Deploy the collector and add the relations to the MicroCloud model:

    juju deploy opentelemetry-collector
    juju relate opentelemetry-collector microcloud:cos-agent
    juju relate opentelemetry-collector microcloud:logging
    juju relate opentelemetry-collector prometheus
    juju relate opentelemetry-collector loki
    juju relate opentelemetry-collector grafana

## Retain Loki log labels

When sending the logs through the OTLP endpoint, an `opentelemetry-collector` global processor config is required to ensure the log labels are retained
when the log gets forwarded to COS.

Run the following command to add the config to the `opentelemetry-collector` charm:

    juju config opentelemetry-collector processors="$(cat <<'EOF'
    attributes/lxd-loki-labels:
    actions:
        - action: upsert
          key: loki.attribute.labels
          value: "instance, app, location, name, project, type"
    EOF
    )"

## Verify the integration

Run the following command to confirm the status of the integration:

    juju status --integrations

The `--integrations` flag ensures the output contains the list of relations made in the current model.

After setting up the relation between the MicroCloud charm and COS, each unit's `Message` should display `Observability active`.
A missing relation yields `Waiting for observability relation` for the respective unit.

Once the relations are established and data has been scraped, open Grafana and look for the bundled LXD, MicroCeph, and MicroOVN dashboards to view your MicroCloud metrics and logs. The bundled alert rules can also be viewed through Grafana.

## Add a cluster member

To add a cluster member, first {ref}`add the machine to the MicroCloud cluster <howto-member-add>`, then add the corresponding Juju machine and charm unit:

    juju add-machine ssh:<user>@<newhost>
    juju add-unit microcloud --to <new-machine-id>

If the new machine is not part of a MicroCloud cluster, then the new unit reports an error.
