#!/bin/bash

test_upgrade() {
  reset_systems 4 2 3

  # Clear out the debug binaries as we want to test upgrades from published versions.
  for i in $(seq -f "%02g" 1 4) ; do
    name="micro${i}"
    if [ -n "${MICROCLOUD_DEBUG_PATH}" ] || [ -n "${MICROCLOUDD_DEBUG_PATH}" ]; then
      lxc exec "${name}" -- rm -f /var/snap/microcloud/common/microcloud.debug || true
      lxc exec "${name}" -- rm -f /var/snap/microcloud/common/microcloudd.debug || true
      lxc exec "${name}" -- systemctl restart snap.microcloud.daemon || true
    fi

    if [ -n "${LXD_DEBUG_PATH}" ]; then
      lxc exec "${name}" -- rm -f /var/snap/lxd/common/lxd.debug
      lxc exec "${name}" -- systemctl reload snap.lxd.daemon || true
      lxc exec "${name}" -- lxd waitready
    fi
  done

  # Perform upgrade test from MicroCloud 1 to 2.
  if [ "${MICROCLOUD_SNAP_CHANNEL}" = "1/stable" ]; then
    microceph_target="squid/stable"
    microovn_target="24.03/stable"
    lxd_target="5.21/stable"
    microcloud_target="2/edge"

    # The lookup subnet has to contain the netmask and the address has to be the one used by MicroCloud not the gateway.
    lookup_subnet="$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address + "/" + .netmask')"

    # Use the MicroCloud version 1 preseed format with all the features available at the time of release:
    # - local and remote storage
    # - wiping disks
    # - OVN uplink interface
    preseed="lookup_subnet: ${lookup_subnet}
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
- name: micro02
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
- name: micro03
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64"

    # Deactivate the fourth MicroCloud so we can add it later after upgrade.
    lxc exec micro04 -- snap stop microcloud

    # Deploy a version 1 MicroCloud and launch some instances.
    lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "$preseed"

    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM LAUNCH TEST"
    else
      lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 v1 --vm
      lxc exec micro01 -- lxc exec v1 -- apt-get update
      lxc exec micro01 -- lxc exec v1 -- apt-get install --no-install-recommends -y iputils-ping
      v1_boot_id=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
    fi
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 c1
    lxc exec micro01 -- lxc exec c1 -- apt-get update
    lxc exec micro01 -- lxc exec c1 -- apt-get install --no-install-recommends -y iputils-ping
    c1_boot_id="$(lxc exec micro01 -- lxc exec c1 -- cat /proc/sys/kernel/random/boot_id)"

    # First upgrade MicroCeph.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microceph --channel "${microceph_target}"
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "MicroCeph member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- microceph cluster list | grep "${m}" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done

      # There was no encryption neither dedicated Ceph networks and CephFS in MicroCloud 1.
      validate_system_microceph "${m}" 0 0 disk2
    done

    # Second upgrade MicroOVN.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microovn --channel "${microovn_target}"
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "MicroOVN member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- microovn cluster list -f json | jq -r ".[] | select(.name == \"${m}\") | .status" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done

      # There was no explicit OVN underlay subnet configuration in MicroCloud 1.
      validate_system_microovn "${m}"
    done

    # Third upgrade LXD.
    for m in micro01 micro02 micro03; do
      # Upgrade them in parallel as the refresh waits for the others to update too.
      lxc exec "${m}" -- snap refresh lxd --channel "${lxd_target}" &
    done
    wait

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "LXD member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- lxc cluster list -f json | jq -r ".[] | select(.server_name == \"${m}\") | .status" | grep -q Online; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done

      # Don't test for DNS nameservers on the OVN network as those weren't yet added in MicroCloud 1.
      validate_system_lxd "${m}" 3 disk1 1 0 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    done

    # Fourth upgrade MicroCloud.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microcloud --channel "${microcloud_target}"
      set_debug_binaries "${m}"
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "LXD member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" --env TEST_CONSOLE=0 -- microcloud cluster list -f json | jq -r ".[] | select(.name == \"${m}\") | .status" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    # Check if the workload survived the upgrade.
    # Perform some pings to check the network.
    gateway="$(lxc network get lxdbr0 ipv4.address | awk -F'/' '{print $1}')"
    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM TESTS"
    else
      lxc exec micro01 -- lxc exec v1 -- ping -nc2 "${gateway}"

      # Compare the boot ids to verify the instances haven't been restarted.
      v1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
      [ "${v1_boot_id}" = "${v1_boot_id_after_upgrade}" ]
    fi
    lxc exec micro01 -- lxc exec c1 -- ping -nc2 "${gateway}"
    c1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec c1 -- cat /proc/sys/kernel/random/boot_id)
    [ "${c1_boot_id}" = "${c1_boot_id_after_upgrade}" ]

    # Upgrade micro04.
    lxc exec micro04 -- snap refresh microceph --channel "${microceph_target}"
    lxc exec micro04 -- snap refresh microovn --channel "${microovn_target}"
    lxc exec micro04 -- snap refresh lxd --channel "${lxd_target}"
    lxc exec micro04 -- snap refresh microcloud --channel "${microcloud_target}"
    lxc exec micro04 -- snap start microcloud
    set_debug_binaries "micro04"

    # Join micro04 to the old cluster using the MicroCloud 2 preseed format.
    lookup_gateway=$(lxc network get lxdbr0 ipv4.address)
    preseed="lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true"

    lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    validate_system_microceph micro04 0 0 disk2
    validate_system_microovn micro04
    validate_system_lxd micro04 4 disk1 1 0 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster list -f json | jq -r '.[] | select(.name == "micro04") | .status' | grep -q ONLINE
  fi
}
