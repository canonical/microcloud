#!/bin/bash

test_upgrade() {
  reset_systems 4 3 5

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

  # Perform upgrade test from MicroCloud 2 to 3.
  if [ "${MICROCLOUD_SNAP_CHANNEL}" = "2/candidate" ]; then
    # Use the edge channels to catch issues early in the release process.
    microceph_target="squid/edge"
    microovn_target="latest/edge"
    lxd_target="6/edge"
    microcloud_target="3/edge"

    addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

    ceph_cluster_subnet_prefix="10.0.1"
    ceph_cluster_subnet_iface="enp7s0"
    ceph_public_subnet_prefix="10.0.2"
    ceph_public_subnet_iface="enp8s0"
    ovn_underlay_subnet_prefix="10.0.3"
    ovn_underlay_subnet_iface="enp9s0"
    set_cluster_subnet 4  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
    set_cluster_subnet 4  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
    set_cluster_subnet 4  "${ovn_underlay_subnet_iface}" "${ovn_underlay_subnet_prefix}"

    # Use the MicroCloud version 2 preseed format with all the features available at the time of release:
    # - Local and remote storage
    # - Wiping disks
    # - Encrypting disks
    # - OVN uplink interface, DNS servers, disaggregated
    # - CephFS
    # - Ceph network disaggregated
    preseed="lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.2
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true
- name: micro02
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.3
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true
- name: micro03
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.4
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1

ceph:
  internal_network: ${ceph_cluster_subnet_prefix}.0/24
  public_network: ${ceph_public_subnet_prefix}.0/24
  cephfs: true"

    # On 22.04 machines we cannot use dm-crypt.
    # Therefore negate the instruction to encrypt the Ceph OSD disks.
    if [ "${BASE_OS}" = "22.04" ]; then
      preseed="$(echo "${preseed}" | yq -e '.systems.[].storage.ceph.[].encrypt = false')"
    fi

    # Deploy a version 2 MicroCloud and launch some instances.
    lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
    lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM LAUNCH TEST"
    else
      lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 v1 --vm
      lxc exec micro01 -- sh -c "$(declare -f waitInstanceReady); waitInstanceReady v1"
      v1_boot_id=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
    fi
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 c1
    lxc exec micro01 -- sh -c "$(declare -f waitInstanceReady); waitInstanceReady c1"
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
    done

    disks_encrypted="1"
    disks_encrypted_list="disk2"

    # On 22.04 machines the check should indicate no Ceph OSD encryption.
    if [ "${BASE_OS}" = "22.04" ]; then
      disks_encrypted="0"
      disks_encrypted_list=""
    fi

    for m in micro01 micro02 micro03; do
      validate_system_microceph "${m}" 1 "${disks_encrypted}" "${ceph_cluster_subnet_prefix}.0/24" "${ceph_public_subnet_prefix}.0/24" disk2 "${disks_encrypted_list}"
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
    done

    for m in micro01 micro02 micro03; do
      validate_system_microovn ${m} "${ovn_underlay_subnet_prefix}"
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
    done

    # Test LXD only after all the cluster members have stabilized after upgrade.
    for m in micro01 micro02 micro03; do
      validate_system_lxd "${m}" 3 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
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
          echo "MicroCloud member ${m} failed to come up after upgrade"
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
    gateway="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM TESTS"
    else
      lxc exec micro01 -- lxc exec v1 -- timeout 5 bash -cex "grep -qm1 ^SSH- < /dev/tcp/${gateway}/22"

      # Compare the boot ids to verify the instances haven't been restarted.
      v1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
      [ "${v1_boot_id}" = "${v1_boot_id_after_upgrade}" ]
    fi
    lxc exec micro01 -- lxc exec c1 -- timeout 5 bash -cex "grep -qm1 ^SSH- < /dev/tcp/${gateway}/22"
    c1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec c1 -- cat /proc/sys/kernel/random/boot_id)
    [ "${c1_boot_id}" = "${c1_boot_id_after_upgrade}" ]

    # Upgrade micro04.
    lxc exec micro04 -- snap refresh microceph --channel "${microceph_target}"
    lxc exec micro04 -- snap refresh microovn --channel "${microovn_target}"
    lxc exec micro04 -- snap refresh lxd --channel "${lxd_target}"
    lxc exec micro04 -- snap refresh microcloud --channel "${microcloud_target}"
    set_debug_binaries "micro04"

    # Join micro04 to the old cluster using the MicroCloud 2 preseed format.
    preseed="lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.5
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true"

    if [ "${BASE_OS}" = "22.04" ]; then
      preseed="$(echo "${preseed}" | yq -e '.systems.[].storage.ceph.[].encrypt = false')"
    fi

    lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    validate_system_microceph micro04 1 "${disks_encrypted}" "${ceph_cluster_subnet_prefix}.0/24" "${ceph_public_subnet_prefix}.0/24" disk2 "${disks_encrypted_list}"
    validate_system_microovn micro04 "${ovn_underlay_subnet_prefix}"
    validate_system_lxd micro04 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
    lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster list -f json | jq -r '.[] | select(.name == "micro04") | .status' | grep -q ONLINE
  fi
}
