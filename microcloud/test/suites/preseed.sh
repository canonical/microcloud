#!/bin/bash

test_preseed() {
  reset_systems 4 3 2
  lookup_addr="$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)/24"

  # Create a MicroCloud with storage directly given by-path on one node, and by filter on other nodes.
preseed="
lookup_subnet: ${lookup_addr}
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
- name: micro02
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
        wipe: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
- name: micro03
  ovn_uplink_interface: enp6s0

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64

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
    - find: device_id == *lxd_disk3
      find_min: 2
      find_max: 2
      wipe: true
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"

  for m in micro01 micro03 ; do
    validate_system_lxd ${m} 3 disk1 2 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    validate_system_microceph ${m} disk2 disk3
    validate_system_microovn ${m}
  done

  # Disks on micro02 should have been manually selected.
  validate_system_lxd micro02 3 disk2 2 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro02 disk1 disk3
  validate_system_microovn micro02

  # Grow the MicroCloud with a new node, with filter-based storage selection.
preseed="
lookup_subnet: ${lookup_addr}
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
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
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud add --preseed <<< "${preseed}"
  validate_system_lxd micro04 4 disk1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
  validate_system_microceph micro04 disk2
  validate_system_microovn micro04

  reset_systems 3 3 2

  # Create a MicroCloud but don't set up storage or network (Should get a FAN setup).
preseed="
lookup_subnet: ${lookup_addr}
systems:
- name: micro01
- name: micro02
- name: micro03
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"
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

preseed="
lookup_subnet: ${lookup_addr}
systems:
- name: micro01
- name: micro02
- name: micro03
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd ${m} 3
  done
}
