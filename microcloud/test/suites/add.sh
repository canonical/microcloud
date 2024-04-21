#!/bin/bash

test_add_auto() {
  reset_systems 4 0 0

  # Test with just LXD and MicroCloud, and no disks.
  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap disable microceph || true
  done

  # Disable extra nodes so we don't add them yet.
  for m in micro03 micro04 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  # Re-enable the nodes.
  for m in micro03 micro04 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  # Add the nodes.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud add --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4

    # Supress the first message from LXD.
    lxc exec "${m}" -- lxc list > /dev/null 2>&1 || true

    # Ensure we created no storage devices.
    [ "$(lxc exec "${m}" -- lxc storage ls -f csv | wc -l)" = "0" ]
  done

  # Test with all systems.
  reset_systems 4 0 0

  # Disable extra nodes so we don't add them yet.
  for m in micro03 micro04 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  # Re-enable the nodes.
  for m in micro03 micro04 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  # Add the nodes.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud add --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Supress the first message from LXD.
    lxc exec "${m}" -- lxc list > /dev/null 2>&1 || true

    # Ensure we created no storage devices.
    [ "$(lxc exec "${m}" -- lxc storage ls -f csv | wc -l)" = "0" ]
  done

  # Test with ZFS and Ceph disks.
  reset_systems 4 2 0

  # Disable extra nodes so we don't add them yet.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  # Re-enable the nodes.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  # Add the nodes.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud add --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 1 0
    validate_system_microceph "${m}" 0 disk2
    validate_system_microovn "${m}"
  done
}

test_add_interactive() {
  reset_systems 4 2 1

  # Disable extra nodes so we don't add them yet.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  echo "Test growing a MicroCloud with all services and devices set up"
  unset_interactive_vars
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_WIPE="yes"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,fd42:1:1234:1234::1"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  # Re-enable the nodes.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  unset_interactive_vars
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=1
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export CEPH_WIPE="yes"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud add > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64  10.1.123.1,fd42:1:1234:1234::1
    validate_system_microceph "${m}" 1 disk2
    validate_system_microovn "${m}"
  done


  reset_systems 4 2 1
  echo "Test growing a MicroCloud with missing services"

  # Disable optional services on the initial cluster only.
  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap disable microceph || true
  done

  lxc exec micro04 -- snap disable microcloud
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- snap enable microcloud
  lxc exec micro04 -- snap start microcloud

  unset_interactive_vars
  export LIMIT_SUBNET="yes"
  export SKIP_SERVICE=yes
  export EXPECT_PEERS=1
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud add > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1
  done
}
