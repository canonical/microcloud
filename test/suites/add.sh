#!/bin/bash

test_add_interactive() {
  reset_systems 4 2 3

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.0.2"
  ceph_public_subnet_iface="enp8s0"

  for n in $(seq 2 5); do
    public_ip="${ceph_public_subnet_prefix}.${n}/24"
    cluster_ip="${ceph_cluster_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${public_ip}" dev "${ceph_public_subnet_iface}"
    lxc exec "micro0$((n-1))" -- ip addr add "${cluster_ip}" dev "${ceph_cluster_subnet_iface}"
  done

  # Disable extra nodes so we don't add them yet.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  echo "Test growing a MicroCloud with all services and devices set up"
  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_public_subnet_prefix}.0/24"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,fd42:1:1234:1234::1"
  export OVN_UNDERLAY_NETWORK="no"
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  # Re-enable the nodes.
  # shellcheck disable=SC2043
  for m in micro04 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  unset_interactive_vars
  export EXPECT_PEERS=1
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export CEPH_WIPE="yes"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export OVN_UNDERLAY_NETWORK="no"
  join_session add micro01 micro04
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64  10.1.123.1,fd42:1:1234:1234::1
    validate_system_microceph "${m}" 1 "${ceph_cluster_subnet_prefix}.0/24" "${ceph_public_subnet_prefix}.0/24" disk2
    validate_system_microovn "${m}"
  done

  reset_systems 4 2 1
  echo "Test growing a MicroCloud with missing services"
  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export SKIP_SERVICE="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="no"
  export SETUP_CEPH="no"
  export SETUP_OVN="no"

  # Disable optional services on the initial cluster only.
  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap disable microceph || true
  done

  lxc exec micro04 -- snap disable microcloud

  join_session init micro01 micro02 micro03
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  lxc exec micro04 -- snap enable microcloud
  lxc exec micro04 -- snap start microcloud

  unset_interactive_vars
  export SKIP_SERVICE=yes
  export EXPECT_PEERS=1
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  join_session add micro01 micro04
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1
  done

  reset_systems 4 2 1
  echo "Test growing a MicroCloud when storage & networks were not already set up"
  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="no"
  export SETUP_CEPH="no"
  export SETUP_OVN="no"

  lxc exec micro04 -- snap disable microcloud
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  for m in micro01 micro02 micro03; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  unset_interactive_vars
  export EXPECT_PEERS=1
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_WIPE="yes"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,fd42:1:1234:1234::1"
  export REPLACE_PROFILE="yes"
  export OVN_UNDERLAY_NETWORK="no"

  lxc exec micro04 -- snap enable microcloud
  lxc exec micro04 -- snap start microcloud
  join_session add micro01 micro04
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  default_cluster_subnet="$(lxc exec micro01 -- ip -4 -br a show enp5s0 | awk '{print $3}')"
  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64  10.1.123.1,fd42:1:1234:1234::1
    validate_system_microceph "${m}" 1 "${default_cluster_subnet}" disk2
    validate_system_microovn "${m}"
  done
}
