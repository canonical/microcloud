#!/bin/bash

test_upgrade() {
  reset_systems 4 2 3

  # Perform upgrade test from MicroCloud 1 to 2.
  if [ "${MICROCLOUD_SNAP_CHANNEL}" = "1/stable" ]; then
    microceph_target="squid/edge" # TODO: squid/stable when available
    microovn_target="latest/edge" # TODO: 24.03/stable when available
    lxd_target="5.21/edge"
    microcloud_target="latest/edge" # TODO: 2/stable when available

    # The lookup subnet has to contain the netmask and the address has to be the one used by MicroCloud not the gateway.
    lookup_subnet="$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address + "/" + .netmask')"

    # Use the MicroCloud version 1 preseed format with all the features available at the time of release:
    # - local and remote storage
    # - wiping disks
    # - OVN uplink interface
    preseed="$(cat << EOF
lookup_subnet: ${lookup_subnet}
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
  ipv6_gateway: fd42:1:1234:1234::1/64
EOF
    )"

    # Deactivate the fourth MicroCloud so we can add it later after upgrade.
    lxc exec micro04 -- snap stop microcloud

    # Deploy a version 1 MicroCloud and launch some instances.
    lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "$preseed"
    lxc exec micro01 -- lxc launch ubuntu:noble v1 --vm
    lxc exec micro01 -- lxc launch ubuntu:noble c1

    v1_uptime="$(lxc exec micro01 -- lxc exec v1 -- uptime -s)"
    c1_uptime="$(lxc exec micro01 -- lxc exec c1 -- uptime -s)"

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

        if lxc exec "${m}" -- sh -c "microceph cluster list | grep ${m} | grep -q ONLINE"; then
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

        if lxc exec "${m}" -- sh -c "microovn cluster list -f json | jq -r '.[] | select(.name == \"${m}\") | .status' | grep -q ONLINE"; then
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

        if lxc exec "${m}" -- sh -c "lxc cluster list -f json | jq -r '.[] | select(.server_name == \"${m}\") | .status' | grep -q Online"; then
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
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "LXD member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" --env TEST_CONSOLE=0 -- sh -c "microcloud cluster list -f json | jq -r '.[] | select(.name == \"${m}\") | .status' | grep -q ONLINE"; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    # Check if the workload survived the upgrade.
    # Perform some pings to check the network.
    gateway="$(lxc network get lxdbr0 ipv4.address | awk -F'/' '{print $1}')"
    lxc exec micro01 -- lxc exec v1 -- ping -c 2 "${gateway}"
    lxc exec micro01 -- lxc exec c1 -- ping -c 2 "${gateway}"

    # Compare the uptimes to verify the instances haven't been restarted.
    v1_uptime_after_upgrade="$(lxc exec micro01 -- lxc exec v1 -- uptime -s)"
    c1_uptime_after_upgrade="$(lxc exec micro01 -- lxc exec c1 -- uptime -s)"
    [ "${v1_uptime}" = "${v1_uptime_after_upgrade}" ]
    [ "${c1_uptime}" = "${c1_uptime_after_upgrade}" ]

    # Upgrade micro04.
    lxc exec micro04 -- snap refresh microceph --channel "${microceph_target}"
    lxc exec micro04 -- snap refresh microovn --channel "${microovn_target}"
    lxc exec micro04 -- snap refresh lxd --channel "${lxd_target}"
    lxc exec micro04 -- snap refresh microcloud --channel "${microcloud_target}"
    lxc exec micro04 -- snap start microcloud

    # Join micro04 to the old cluster using the MicroCloud 2 preseed format.
    lookup_gateway=$(lxc network get lxdbr0 ipv4.address)
    preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
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
        wipe: true
EOF
    )"

    lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    validate_system_microceph micro04 0 0 disk2
    validate_system_microovn micro04
    validate_system_lxd micro04 4 disk1 1 0 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c "microcloud cluster list -f json | jq -r '.[] | select(.name == \"micro04\") | .status' | grep -q ONLINE"
  fi
}
