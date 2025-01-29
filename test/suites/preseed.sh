#!/bin/bash

cleanup_preseed() {
  # Cleanup child processes sent to the background using &.
  child_processes="$(jobs -pr)"
  if [ -n "${child_processes}" ]; then
    for p in ${child_processes}; do
      kill -9 "${p}"
    done
  fi

  cleanup
}

test_preseed() {
  # Overwrite the regular trap to cleanup background processes.
  trap cleanup_preseed EXIT HUP INT TERM

  reset_systems 4 3 2
  lookup_gateway=$(lxc network get lxdbr0 ipv4.address)

  ceph_cluster_subnet_prefix="10.2.123"
  ceph_cluster_subnet_iface="enp7s0"

  for n in $(seq 2 5); do
    cluster_ip="${ceph_cluster_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${cluster_ip}" dev "${ceph_cluster_subnet_iface}"
  done

  ovn_underlay_subnet_prefix="10.3.123"
  ovn_underlay_subnet_iface="enp8s0"

  for n in $(seq 2 5); do
    ovn_underlay_ip="${ovn_underlay_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- sh -c "ip addr add ${ovn_underlay_ip} dev ${ovn_underlay_subnet_iface} && ip link set ${ovn_underlay_subnet_iface} up"
  done

  # Create a MicroCloud with storage directly given by-path on one node, and by filter on other nodes.
  preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.3.123.2
- name: micro02
  ovn_underlay_ip: 10.3.123.3
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
        wipe: true
        encrypt: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
        encrypt: true
- name: micro03
  ovn_underlay_ip: 10.3.123.4
  ovn_uplink_interface: enp6s0

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1

ceph:
  internal_network: ${ceph_cluster_subnet_prefix}.0/24
  cephfs: true

storage:
  local:
    - find: device_id == *lxd_disk1
      find_min: 2
      find_max: 2
      wipe: true
  ceph:
    - find: device_id == *lxd_disk2
      find_min: 2
      find_max: 2
      wipe: true
      encrypt: true
    - find: device_id == *lxd_disk3
      find_min: 2
      find_max: 2
      wipe: true
      encrypt: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro03 ; do
    validate_system_lxd ${m} 3 disk1 2 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
    validate_system_microceph ${m} 1 1 "${ceph_cluster_subnet_prefix}.0/24" disk2,disk3 disk2 disk3
    validate_system_microovn ${m}  "${ovn_underlay_subnet_prefix}"
  done

  # Disks on micro02 should have been manually selected.
  validate_system_lxd micro02 3 disk2 2 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro02 1 1 "${ceph_cluster_subnet_prefix}.0/24"  disk1,disk3 disk1 disk3
  validate_system_microovn micro02 "${ovn_underlay_subnet_prefix}"

  # Grow the MicroCloud with a new node, with filter-based storage selection.
  preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.3.123.5
ceph:
  cephfs: true
storage:
  local:
    - find: device_id == *lxd_disk1
      find_min: 1
      find_max: 1
      wipe: true
  ceph:
    - find: device_id == *lxd_disk2
      find_min: 1
      find_max: 1
      wipe: true
      encrypt: true
EOF
  )"

  lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  validate_system_lxd micro04 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro04 1 1 "${ceph_cluster_subnet_prefix}.0/24" disk2 disk2
  validate_system_microovn micro04 "${ovn_underlay_subnet_prefix}"

  reset_systems 3 3 2

  # Create a MicroCloud but don't set up storage or network (Should get a FAN setup).
  preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
- name: micro02
- name: micro03
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 micro03 ; do
    validate_system_lxd ${m} 3
    validate_system_microceph ${m}
    validate_system_microovn ${m}
  done

  reset_systems 3 3 2

  # Create a MicroCloud if we don't have MicroOVN or MicroCeph installed.
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap disable microovn
  sleep 1

  preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
- name: micro02
- name: micro03
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 micro03 ; do
    validate_system_lxd ${m} 3
  done

  reset_systems 4 2 1

  # Create a MicroCloud with MicroCeph only across a subset of the members.
  preseed="$(cat << EOF
lookup_subnet: ${lookup_gateway}
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
        wipe: true
        encrypt: true
- name: micro02
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
        wipe: true
        encrypt: true
- name: micro03
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
        wipe: true
        encrypt: true
- name: micro04
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1

ceph:
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 micro03 micro04; do
    validate_system_lxd ${m} 4 disk2 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
    validate_system_microovn ${m}

    # micro04 doesn't contribute an OSD to MicroCeph so skip validation of disks.
    if [ "${m}" = "micro04" ]; then
      validate_system_microceph ${m} 1
    else
      validate_system_microceph ${m} 1 1 disk1 disk1
    fi
  done
}
