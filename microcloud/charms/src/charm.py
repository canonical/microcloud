#!/usr/bin/env python3

"""Microcloud charm."""

import logging
import os
import shutil
import subprocess
import tarfile
import tempfile
import time
from typing import Dict, List, Union

from ops.charm import (
    CharmBase,
    ConfigChangedEvent,
    InstallEvent,
    RelationBrokenEvent,
    RelationCreatedEvent,
    RelationDepartedEvent,
    RelationJoinedEvent,
    StartEvent,
    UpdateStatusEvent,
)
from ops.framework import StoredState
from ops.main import main
from ops.model import (
    ActiveStatus,
    BlockedStatus,
    MaintenanceStatus,
    ModelError,
    WaitingStatus,
)

logger = logging.getLogger(__name__)


class MaasMicrocloudCharmCharm(CharmBase):
    """Microcloud charm class."""

    _stored = StoredState()

    def __init__(self, *args):
        """Initialize charm's variable."""
        super().__init__(*args)

        # Initialize the persistent storage if needed
        self._stored.set_default(
            config={},
            microcloud_binary_path="",
            microcloud_snap_path="",
        )

        # Main event handlers
        self.framework.observe(self.on.install, self._on_charm_install)
        self.framework.observe(self.on.config_changed, self._on_charm_config_changed)
        self.framework.observe(self.on.start, self._on_charm_start)
        self.framework.observe(self.on.update_status, self._on_update_status)

        # Relation event handlers
        self.framework.observe(self.on.cluster_relation_created, self._on_cluster_relation_created)
        self.framework.observe(self.on.cluster_relation_joined, self._on_cluster_relation_joined)
        self.framework.observe(
            self.on.cluster_relation_departed, self._on_cluster_relation_departed
        )
        self.framework.observe(self.on.cluster_relation_broken, self._on_cluster_relation_broken)

    @property
    def peers(self):
        """Fetch the cluster relation."""
        return self.model.get_relation("cluster")

    def get_peer_data_str(self, bag, key: str) -> str:
        """Retrieve a str from the peer data bag."""
        if not self.peers or not bag or not key:
            return ""

        value = self.peers.data[bag].get(key, "")
        if isinstance(value, str):
            return value

        logger.error(f"Invalid data pulled out from {bag.name}.get('{key}')")
        return ""

    def set_peer_data_str(self, bag, key: str, value: str) -> None:
        """Put a str into the peer data bag if not there or different."""
        if not self.peers or not bag or not key:
            return

        old_value: str = self.get_peer_data_str(bag, key)
        if old_value != value:
            self.peers.data[bag][key] = value

    def _on_charm_install(self, event: InstallEvent) -> None:
        logger.info("Installing the Microcloud charm")
        # Confirm that the config is valid
        if not self.config_is_valid():
            return

        # Install Microcloud itself
        try:
            self.snap_install_microcloud()
            logger.info("Microcloud installed successfully")
        except RuntimeError:
            logger.error("Failed to install Microcloud")
            event.defer()
            return

        # Apply sideloaded resources attached at deploy time
        self.resource_sideload()

    def _on_cluster_relation_created(self, event: RelationCreatedEvent) -> None:
        """We must wait for all units to be ready before initializing Microcloud."""
        self.set_peer_data_str(self.unit, "new_node", "True")
        if not self.unit.is_leader():
            time.sleep(30)  # Make sure that the leader has time to initialize the cluster

        return

    def _on_charm_start(self, event: StartEvent) -> None:
        logger.info("Starting the Microcloud charm")

        if self.config_changed():
            logger.debug("Pending config changes detected")
            self._on_charm_config_changed(event)

        one_unit_clustered = False
        for unit in self.peers.units:
            if self.peers.data[unit].get("clustered") == "True":
                one_unit_clustered = True
                break

        if one_unit_clustered:
            # check if this unit has been clustered by the init process
            try:
                subprocess.run(
                    ["lxc", "cluster", "list"],
                    check=True,
                    timeout=600,
                )
                self.set_peer_data_str(self.unit, "clustered", "True")
                self.set_peer_data_str(self.unit, "new_node", "")
                self.unit_active("Healthy Microcloud unit")
                return
            except subprocess.CalledProcessError:
                self.unit_waiting("This unit has not joined the cluster yet")
                event.defer()
                return
            except subprocess.TimeoutExpired:
                self.unit_blocked("This unit timed out checking its clustered status")
                return

        new_peers = [self.peers.data[unit].get("new_node") == "True" for unit in self.peers.units]
        if (
            self.unit.is_leader()
            and self.get_peer_data_str(self.unit, "new_node") == "True"
            and all(new_peers)
            and self.app.planned_units() == len(self.peers.units) + 1
        ):
            try:
                self.microcloud_init()
                self.set_peer_data_str(
                    self.unit, "clustered", "True"
                )  # This unit is sure to be clustered
                self.set_peer_data_str(self.unit, "new_node", "")  # Not a new node anymore
                # TODO: we can't say for sure that the cluster contains self.app.planned_units()
                # some nodes might have failed to join the cluster but the command result
                # is still a code 0. A workaround would be to parse the number of lines
                # of `lxc cluster list -f csv` on this node.
                self.set_peer_data_str(
                    self.app, "num_clustered_units", str(self.app.planned_units())
                )  # Register the size of the cluster. Useful for when we will want to add new nodes
                self.unit_active("Microcloud successfully initialized")
                logger.info("Microcloud successfully initialized")
                return
            except RuntimeError as e:
                logger.error(f"Failed to initialize Microcloud: {e}")
                self.unit_blocked("Failed to initialize Microcloud")
                return

        time.sleep(10)  # Wait a bit before deferring the event

        if self.unit.is_leader():
            logger.warning("Leader needs to wait for all units to be ready to bootstrap")
            self.unit_waiting("Leader needs to wait for all units to be ready to bootstrap")
        else:
            logger.warning("Unit needs to wait for all units to be ready to bootstrap")
            self.unit_waiting("Unit needs to wait for all units to be ready to bootstrap")

        event.defer()
        return

    def _on_update_status(self, event: UpdateStatusEvent) -> None:
        """Regularly check if the unit is clustered."""
        try:
            subprocess.run(
                ["lxc", "cluster", "list"],
                check=True,
                timeout=600,
            )
            self.set_peer_data_str(self.unit, "clustered", "True")
            self.unit_active("Healthy Microcloud unit")
        except subprocess.CalledProcessError:
            self.unit_blocked("This unit has failed to join the cluster")
            return
        except subprocess.TimeoutExpired:
            self.unit_blocked("This unit timed out checking its clustered status")
            return

    def _on_charm_config_changed(self, event: Union[ConfigChangedEvent, StartEvent]) -> None:
        """React to configuration changes. (JuJu refresh)."""
        logger.info("Updating charm config")

        # Confirm that the config is valid
        if not self.config_is_valid():
            return

        # Get all the configs that changed
        changed = self.config_changed()
        if not changed:
            logger.debug("No configuration changes to apply")
            return

        # Apply all the configs that changed
        try:
            if (
                "snap-channel-lxd" in changed
                or "snap-channel-microcloud" in changed
                or "snap-channel-microceph" in changed
                or "snap-channel-microovn" in changed
            ):
                logger.info(
                    "Changes have been detected in the snap channels, updating the snaps..."
                )
                self.snap_install_microcloud()
        except RuntimeError:
            msg = "Failed to apply some configuration change(s): %s" % ", ".join(changed)
            self.unit_blocked(msg)
            event.defer()
            return

    def _on_cluster_relation_joined(self, event: RelationJoinedEvent) -> None:
        """Add a new node to the existing Microcloud cluster."""
        if (
            self.unit.is_leader()
            and self.get_peer_data_str(self.unit, "clustered") == "True"
            and event.unit
            != self.unit  # Don't add the leader to the cluster as it is already there
        ):
            try:
                self.microcloud_add()
                logger.info("New Microcloud node successfully added")
                return
            except RuntimeError:
                logger.error("Failed to add a new Microcloud node")
                return

    def _on_cluster_relation_departed(self, event: RelationDepartedEvent) -> None:
        """Update the application information regarding the number of clustered units."""
        if self.unit.is_leader():
            num_clustered_units = int(self.get_peer_data_str(self.app, "num_clustered_units"))
            self.set_peer_data_str(self.app, "num_clustered_units", str(num_clustered_units - 1))

    def _on_cluster_relation_broken(self, event: RelationBrokenEvent) -> None:
        """Effectively remove this node from the existing Microcloud cluster."""
        if self.get_peer_data_str(self.unit, "clustered") == "True":
            try:
                self.microcloud_remove(os.uname().nodename)
                logger.info("Microcloud node successfully removed")
            except RuntimeError:
                logger.error("Failed to remove a Microcloud node, retrying later")

    def config_changed(self) -> Dict:
        """Figure out what changed."""
        new_config = self.config
        old_config = self._stored.config
        apply_config = {}
        for k, v in new_config.items():
            if k not in old_config:
                apply_config[k] = v
            elif v != old_config[k]:
                apply_config[k] = v

        return apply_config

    def config_is_valid(self) -> bool:
        """Validate the config."""
        config_changed = self.config_changed()
        logger.info(f"Validating config: {config_changed}")

        # TODO: For now, we don't have any config to validate
        return True

    def microcloud_init(self) -> None:
        """Apply initial configuration of Microcloud."""
        self.unit_maintenance("Initializing Microcloud")

        try:
            microcloud_process_init = subprocess.run(
                ["microcloud", "init", "--auto"],
                capture_output=True,
                check=True,
                timeout=600,
                text=True,
            )

            subprocess.run(
                ["microceph", "enable", "rgw"],
                check=True,
                timeout=600,
            )

            logger.info(f"Microcloud successfully initialized:\n{microcloud_process_init.stdout}")
        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError
        except subprocess.TimeoutExpired as e:
            self.unit_blocked(f'Timeout exceeded while running "{e.cmd}"')
            raise RuntimeError

    def microcloud_add(self) -> None:
        """Add a new node to Microcloud."""
        self.unit_maintenance("Adding node to Microcloud")

        try:
            microcloud_process_add = subprocess.run(
                ["microcloud", "add", "--auto"],
                capture_output=True,
                check=True,
                timeout=600,
                text=True,
            )
            logger.info(f"Microcloud node(s) successfully added:\n{microcloud_process_add.stdout}")
        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError
        except subprocess.TimeoutExpired as e:
            self.unit_blocked(f'Timeout exceeded while running "{e.cmd}"')
            raise RuntimeError

    def microcloud_remove(self, node_name_to_remove: str) -> None:
        """Remove a node from Microcloud."""
        self.unit_maintenance("Removing node from Microcloud")

        try:
            subprocess.run(
                ["microcloud", "cluster", "remove", node_name_to_remove],
                capture_output=True,
                check=True,
                timeout=600,
                text=True,
            )
            logger.info("Microcloud node(s) successfully removed")
        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError
        except subprocess.TimeoutExpired as e:
            self.unit_blocked(f'Timeout exceeded while running "{e.cmd}"')
            raise RuntimeError

    def snap_install_microcloud(self) -> None:
        """Install Microcloud from snap."""
        lxd_channel = self.config["snap-channel-lxd"]
        if lxd_channel:
            lxd_channel_name = lxd_channel
        else:
            lxd_channel_name = "latest/stable"
        self.unit_maintenance(f"Installing LXD snap (channel={lxd_channel_name})")

        microcloud_channel = self.config["snap-channel-microcloud"]
        if microcloud_channel:
            microcloud_channel_name = microcloud_channel
        else:
            microcloud_channel_name = "latest/stable"
        self.unit_maintenance(f"Installing Microcloud snap (channel={microcloud_channel_name})")

        microceph = self.config["microceph"]
        if microceph:
            microceph_channel = self.config["snap-channel-microceph"]
            if microceph_channel:
                microceph_channel_name = microceph_channel
            else:
                microceph_channel_name = "latest/stable"
            self.unit_maintenance(f"Installing Microceph snap (channel={microceph_channel_name})")

        microovn = self.config["microovn"]
        if microovn:
            microovn_channel = self.config["snap-channel-microovn"]
            if microovn_channel:
                microovn_channel_name = microovn_channel
            else:
                microovn_channel_name = "latest/stable"
            self.unit_maintenance(f"Installing MicroOVN snap (channel={microovn_channel_name})")

        try:
            self.unit_maintenance("Refreshing snapd...")
            subprocess.run(
                ["snap", "refresh"],
                capture_output=True,
                check=True,
                timeout=600,
            )
            self.unit_maintenance("snapd refreshed successfully.")
            self.unit_maintenance("Installing core core20 core22...")
            cohort = ["--cohort=+"]
            subprocess.run(
                ["snap", "install", "core", "core20", "core22"] + cohort,
                capture_output=True,
                check=True,
                timeout=600,
            )
            self.unit_maintenance("core core20 core22 installed successfully...")

            # LXD
            self.unit_maintenance("Installing LXD...")
            subprocess.run(
                ["snap", "install", "lxd", f"--channel={lxd_channel}"] + cohort,
                capture_output=True,
                check=True,
                timeout=600,
            )
            subprocess.run(
                ["snap", "refresh", "lxd", f"--channel={lxd_channel}"] + cohort,
                capture_output=True,
                check=True,
                timeout=600,
            )
            self.unit_maintenance("LXD installed successfully.")
            if os.path.exists("/var/lib/lxd"):
                subprocess.run(
                    ["lxd.migrate", "-yes"], capture_output=True, check=True, timeout=600
                )

            # Microcloud
            self.unit_maintenance("Installing Microcloud...")
            subprocess.run(
                ["snap", "install", "microcloud", f"--channel={microcloud_channel}"] + cohort,
                capture_output=True,
                check=True,
                timeout=600,
            )
            subprocess.run(
                ["snap", "refresh", "microcloud", f"--channel={microcloud_channel}"] + cohort,
                capture_output=True,
                check=True,
                timeout=600,
            )
            self.unit_maintenance("Microcloud installed successfully.")

            # MicroCeph
            if microceph:
                self.unit_maintenance("Installing Microceph...")
                subprocess.run(
                    ["snap", "install", "microceph", f"--channel={microceph_channel}"] + cohort,
                    capture_output=True,
                    check=True,
                    timeout=600,
                )
                subprocess.run(
                    ["snap", "refresh", "microceph", f"--channel={microceph_channel}"] + cohort,
                    capture_output=True,
                    check=True,
                    timeout=600,
                )

                subprocess.run(
                    ["rm", "-rf", "/etc/ceph"],
                    check=True,
                    timeout=600,
                )
                subprocess.run(
                    ["ln", "-s", "/var/snap/microceph/current/conf/", "/etc/ceph"],
                    check=True,
                    timeout=600,
                )

                self.unit_maintenance("Microceph installed successfully.")

            # MicroOVN
            if microovn:
                self.unit_maintenance("Installing Microovn...")
                subprocess.run(
                    ["snap", "install", "microovn"] + cohort,
                    capture_output=True,
                    check=True,
                    timeout=600,
                )
                subprocess.run(
                    ["snap", "refresh", "microovn"] + cohort,
                    capture_output=True,
                    check=True,
                    timeout=600,
                )
                self.unit_maintenance("Microovn installed successfully.")
        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError
        except subprocess.TimeoutExpired as e:
            self.unit_blocked(f'Timeout exceeded while running "{e.cmd}"')
            raise RuntimeError

        # Done with the snap installation
        self._stored.config["snap-channel-lxd"] = lxd_channel
        self._stored.config["snap-channel-microcloud"] = microcloud_channel
        if microceph:
            self._stored.config["snap-channel-microceph"] = microceph_channel
        if microovn:
            self._stored.config["snap-channel-microovn"] = microovn_channel

    def microcloud_reload(self) -> None:
        """Reload the microcloud daemon."""
        self.unit_maintenance("Reloading Microcloud")
        try:
            # Avoid occasional race during startup where a reload could cause a failure
            subprocess.run(
                ["microcloud", "waitready", "--timeout=30"], capture_output=True, check=False
            )
            subprocess.run(
                ["systemctl", "reload", "snap.microcloud.daemon.service"],
                capture_output=True,
                check=True,
            )

        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError

    def resource_sideload(self) -> None:
        """Sideload resources."""
        # Multi-arch support
        arch: str = os.uname().machine
        possible_archs: List[str] = [arch]
        if arch == "x86_64":
            possible_archs = ["x86_64", "amd64"]

        # Microcloud snap
        microcloud_snap_resource: str = ""
        fname_suffix: str = ".snap"
        try:
            # Note: self._stored can only store simple data types (int/float/dict/list/etc)
            microcloud_snap_resource = str(self.model.resources.fetch("microcloud-snap"))
        except ModelError:
            pass

        tmp_dir: str = ""
        if microcloud_snap_resource and tarfile.is_tarfile(microcloud_snap_resource):
            logger.debug(f"{microcloud_snap_resource} is a tarball; unpacking")
            tmp_dir = tempfile.mkdtemp()
            tarball = tarfile.open(microcloud_snap_resource)
            valid_names = {f"microcloud_{x}{fname_suffix}" for x in possible_archs}
            for f in valid_names.intersection(tarball.getnames()):
                tarball.extract(f, path=tmp_dir)
                logger.debug(f"{f} was extracted from the tarball")
                self._stored.lxd_snap_path = f"{tmp_dir}/{f}"
                break
            else:
                logger.debug("Missing arch specific snap from tarball.")
            tarball.close()
        else:
            self._stored.microcloud_snap_path = microcloud_snap_resource

        if self._stored.microcloud_snap_path:
            self.snap_sideload_microcloud()
            if tmp_dir:
                os.remove(self._stored.microcloud_snap_path)
                os.rmdir(tmp_dir)

        # Microcloud binary
        microcloud_binary_resource: str = ""
        fname_suffix = ""
        try:
            # Note: self._stored can only store simple data types (int/float/dict/list/etc)
            microcloud_binary_resource = str(self.model.resources.fetch("microcloud-binary"))
        except ModelError:
            pass

        tmp_dir = ""
        if microcloud_binary_resource and tarfile.is_tarfile(microcloud_binary_resource):
            logger.debug(f"{microcloud_binary_resource} is a tarball; unpacking")
            tmp_dir = tempfile.mkdtemp()
            tarball = tarfile.open(microcloud_binary_resource)
            valid_names = {f"microcloud_{x}{fname_suffix}" for x in possible_archs}
            for f in valid_names.intersection(tarball.getnames()):
                tarball.extract(f, path=tmp_dir)
                logger.debug(f"{f} was extracted from the tarball")
                self._stored.microcloud_binary_path = f"{tmp_dir}/{f}"
                break
            else:
                logger.debug("Missing arch specific binary from tarball.")
            tarball.close()
        else:
            self._stored.microcloud_binary_path = microcloud_binary_resource

        if self._stored.microcloud_binary_path:
            self.snap_sideload_microcloud_binary()
            if tmp_dir:
                os.remove(self._stored.microcloud_binary_path)
                os.rmdir(tmp_dir)

    def snap_sideload_microcloud(self) -> None:
        """Sideload Microcloud snap resource."""
        logger.debug("Applying Microcloud snap sideload changes")

        cmd: List[str] = []
        alias: List[str] = []
        enable: List[str] = []

        # A 0 byte file will unload the resource
        if os.path.getsize(self._stored.microcloud_snap_path) == 0:
            logger.debug("Reverting to Microcloud snap from snapstore")
            channel: str = self._stored.config["snap-channel-microcloud"]
            cmd = ["snap", "refresh", "microcloud", f"--channel={channel}", "--amend"]
        else:
            logger.debug("Sideloading Microcloud snap")
            cmd = ["snap", "install", "--dangerous", self._stored.microcloud_snap_path]
            # Since the sideloaded snap doesn't have an assertion, some things need
            # to be done manually
            enable = ["systemctl", "enable", "--now", "snap.microcloud.daemon.unix.socket"]

        try:
            subprocess.run(cmd, capture_output=True, check=True, timeout=600)
            if alias:
                subprocess.run(alias, capture_output=True, check=True, timeout=600)
            if enable:
                subprocess.run(enable, capture_output=True, check=True, timeout=600)
        except subprocess.CalledProcessError as e:
            self.unit_blocked(f'Failed to run "{e.cmd}": {e.stderr} ({e.returncode})')
            raise RuntimeError
        except subprocess.TimeoutExpired as e:
            self.unit_blocked(f'Timeout exceeded while running "{e.cmd}"')
            raise RuntimeError

    def snap_sideload_microcloud_binary(self) -> None:
        """Sideload Microcloud binary resource."""
        logger.debug("Applying Microcloud binary sideload changes")
        microcloud_debug: str = "/var/snap/microcloud/common/microcloud.debug"

        # A 0 byte file will unload the resource
        if os.path.getsize(self._stored.lxd_binary_path) == 0:
            logger.debug("Unloading sideloaded Microcloud binary")
            if os.path.exists(microcloud_debug):
                os.remove(microcloud_debug)
        else:
            logger.debug("Sideloading Microcloud binary")
            # Avoid "Text file busy" error
            if os.path.exists(microcloud_debug):
                logger.debug("Removing old sideloaded LXD binary")
                os.remove(microcloud_debug)
            shutil.copyfile(self._stored.microcloud_binary_path, microcloud_debug)
            os.chmod(microcloud_debug, 0o755)

        self.microcloud_reload()

    def unit_active(self, msg: str = "") -> None:
        """Set the unit's status to active and log the provided message, if any."""
        self.unit.status = ActiveStatus()
        if msg:
            logger.debug(msg)

    def unit_blocked(self, msg: str) -> None:
        """Set the unit's status to blocked and log the provided message."""
        self.unit.status = BlockedStatus(msg)
        logger.error(msg)

    def unit_maintenance(self, msg: str) -> None:
        """Set the unit's status to maintenance and log the provided message."""
        self.unit.status = MaintenanceStatus(msg)
        logger.info(msg)

    def unit_waiting(self, msg: str) -> None:
        """Set the unit's status to waiting and log the provided message."""
        self.unit.status = WaitingStatus(msg)
        logger.info(msg)


if __name__ == "__main__":
    main(MaasMicrocloudCharmCharm)
