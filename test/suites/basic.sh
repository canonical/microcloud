#!/bin/bash

test_interactive() {
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with all services but no devices"
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="no"
  export SETUP_CEPH="no"
  export SETUP_OVN_EXPLICIT="no"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  # Reset the systems with just LXD.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with ZFS storage"
  export SKIP_SERVICE="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  unset SETUP_CEPH SETUP_OVN_EXPLICIT
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems with just LXD and no IPv6 support.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- sh -c "echo 1 > /proc/sys/net/ipv6/conf/all/disable_ipv6"
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

 # Unset the lookup interface because we don't have multiple addresses to select from anymore.
  unset LOOKUP_IFACE

  echo "Creating a MicroCloud with ZFS storage and no IPv6 support"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems with no IPv4 support.
  gw_net_addr=$(lxc network get lxdbr0 ipv4.address)
  lxc network set lxdbr0 ipv4.address none
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap restart microcloud
  done

  # As there is no remote networking, deploy a single node local MicroCloud
  export MULTI_NODE="no"
  export SKIP_LOOKUP=1

  # This will avoid to setup the cluster if no IPv4 overlay networking is available.
  export SETUP_CEPH="no"
  export SETUP_OVN_EXPLICIT="no"
  export PROCEED_WITH_NO_OVERLAY_NETWORKING="no"

  echo "Creating a MicroCloud with ZFS storage and no IPv4 support"
  ! join_session init micro01 || false

  # Ensure we error out due to a lack of usable overlay networking.
  lxc exec micro01 -- cat out | grep "Cluster bootstrapping aborted due to lack of usable networking" -q

  # Set the IPv4 address back to the original value.
  lxc network set lxdbr0 ipv4.address "${gw_net_addr}"
  unset PROCEED_WITH_NO_OVERLAY_NETWORKING SKIP_LOOKUP SETUP_CEPH SETUP_OVN_EXPLICIT

  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"

  # Reset the systems and install microceph.
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

  echo "Creating a MicroCloud with ZFS and Ceph storage"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_ENCRYPT="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 1 1
    validate_system_microceph "${m}" 1 disk2
  done

  # Reset the systems and install microovn.
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap restart microcloud
  done

  echo "Creating a MicroCloud with ZFS storage and OVN network"
  unset SETUP_CEPH CEPH_FILTER CEPH_WIPE SETUP_CEPHFS

  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export OVN_UNDERLAY_NETWORK="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 0 0 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph.
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with ZFS and Ceph storage, and OVN network"
  unset SKIP_SERVICE
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_ENCRYPT="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
    validate_system_microceph "${m}" 1 disk2
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph (with Ceph encryption).
  reset_systems 3 3 1

  echo "Creating a MicroCloud with ZFS and Ceph storage, and OVN network with Ceph encryption"
  export CEPH_ENCRYPT="yes"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
    # Check ceph encryption for disk2 as part of the microceph validation.
    validate_system_microceph "${m}" 1 1 disk2 disk2
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph with a partially disaggregated ceph network setup.
  reset_systems 3 3 2

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"

  for n in $(seq 2 4); do
    cluster_ip="${ceph_cluster_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${cluster_ip}" dev "${ceph_cluster_subnet_iface}"
  done

  echo "Creating a MicroCloud with ZFS, Ceph storage with a partially disaggregated Ceph networking setup, and OVN network"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export OVN_UNDERLAY_NETWORK="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" 1 "${CEPH_CLUSTER_NETWORK}" "${CEPH_PUBLIC_NETWORK}" disk2
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph with a fully disaggregated ceph network setup.
  reset_systems 3 3 3

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.0.2"
  ceph_public_subnet_iface="enp8s0"

  for n in $(seq 2 4); do
    cluster_ip="${ceph_cluster_subnet_prefix}.${n}/24"
    public_ip="${ceph_public_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${cluster_ip}" dev "${ceph_cluster_subnet_iface}"
    lxc exec "micro0$((n-1))" -- ip addr add "${public_ip}" dev "${ceph_public_subnet_iface}"
  done

  echo "Creating a MicroCloud with ZFS, Ceph storage with a fully disaggregated Ceph networking setup, and OVN network"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_public_subnet_prefix}.0/24"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export OVN_UNDERLAY_NETWORK="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" 1 "${CEPH_CLUSTER_NETWORK}" "${CEPH_PUBLIC_NETWORK}" disk2
    validate_system_microovn "${m}"
  done

  reset_systems 3 3 4

  ceph_cluster_subnet_prefix="10.2.123"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.3.123"
  ceph_public_subnet_iface="enp8s0"

  for n in $(seq 2 4); do
    cluster_ip="${ceph_cluster_subnet_prefix}.${n}/24"
    public_ip="${ceph_public_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${cluster_ip}" dev "${ceph_cluster_subnet_iface}"
    lxc exec "micro0$((n-1))" -- ip addr add "${public_ip}" dev "${ceph_public_subnet_iface}"
  done

  ovn_underlay_subnet_prefix="10.4.123"
  ovn_underlay_subnet_iface="enp9s0"

  for n in $(seq 2 4); do
    ovn_underlay_ip="${ovn_underlay_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- sh -c "ip addr add ${ovn_underlay_ip} dev ${ovn_underlay_subnet_iface} && ip link set ${ovn_underlay_subnet_iface} up"
  done

  echo "Creating a MicroCloud with ZFS, Ceph storage with a fully disaggregated Ceph networking setup, OVN management network and OVN underlay network"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_public_subnet_prefix}.0/24"
  export OVN_UNDERLAY_NETWORK="yes"
  export OVN_UNDERLAY_FILTER="${ovn_underlay_subnet_prefix}"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" 1 "${CEPH_CLUSTER_NETWORK}" "${CEPH_PUBLIC_NETWORK}" disk2
    validate_system_microovn "${m}" "${ovn_underlay_subnet_prefix}"
  done

  reset_systems 3 3 2
  echo "Add a MicroCloud node on a cluster of 2 nodes with MicroOVN not initialized"

  ovn_underlay_subnet_prefix="10.2.123"
  ovn_underlay_subnet_iface="enp7s0"

  for n in $(seq 2 4); do
    ovn_underlay_ip="${ovn_underlay_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- sh -c "ip addr add ${ovn_underlay_ip} dev ${ovn_underlay_subnet_iface} && ip link set ${ovn_underlay_subnet_iface} up"
  done

  lxc exec micro03 -- snap disable microcloud || true
  for m in micro01 micro02 ; do
    lxc exec "${m}" -- snap disable microovn
    lxc exec "${m}" -- snap restart microcloud
  done

  unset_interactive_vars
  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=1
  export SKIP_SERVICE="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="no"
  export SETUP_OVN_EXPLICIT="no"

  # Run a 2 nodes MicroCloud without MicroOVN first.
  join_session init micro01 micro02

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 ; do
    lxc exec "${m}" -- snap enable microovn
    lxc exec "${m}" -- snap restart microcloud
  done

  lxc exec micro03 -- microovn cluster bootstrap
  lxc exec micro03 -- snap enable microcloud
  lxc exec micro03 -- snap start microcloud

  unset_interactive_vars
  export EXPECT_PEERS=1
  export PEERS_FILTER="micro03"
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="no"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export OVN_UNDERLAY_NETWORK="yes"
  export OVN_UNDERLAY_FILTER="${ovn_underlay_subnet_prefix}"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export REPLACE_PROFILE="yes"
  join_session add micro01 micro03

  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 0 0 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
  done

  for m in micro01 micro02 ; do
    validate_system_microovn "${m}" "${ovn_underlay_subnet_prefix}"
  done

  # Initiate a MicroCloud cluster (with no uplink interface on the nodes)
  # but abort the setup from the initiator and check that the joiners stop as well.
  reset_systems 3 3 0
  unset_interactive_vars

  echo "Initiate a MicroCloud cluster but abort the setup from the initiator and check that the joiners stop as well"
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="no"
  export OVN_WARNING="no"

  ! join_session init micro01 micro02 micro03 || false
  lxc exec micro01 -- tail -1 out | grep "User aborted" -q
  lxc exec micro02 -- tail -1 out | grep "Failed waiting during join: Initiator aborted the setup" -q
  lxc exec micro03 -- tail -1 out | grep "Failed waiting during join: Initiator aborted the setup" -q

  echo "Initiate a MicroCloud cluster, grow it with a new node, and abort the setup from the initiator and check that the joiners stop as well"
  reset_systems 4 0 0

  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export OVN_WARNING="yes"

  join_session init micro01 micro02 micro03
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  unset_interactive_vars
  export EXPECT_PEERS=1
  export LOOKUP_IFACE="enp5s0"
  export OVN_WARNING="no"
  ! join_session add micro01 micro04 || false
  lxc exec micro01 -- tail -1 out | grep "User aborted" -q
  lxc exec micro04 -- tail -1 out | grep "Failed waiting during join: Initiator aborted the setup" -q

  echo "Try to initiate a MicroCloud with left over networking resources and check it fails to configure OVN"
  reset_systems 3 2 1

  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_ENCRYPT="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export OVN_WARNING="yes" # causes fallback to FAN networking

  # Fake left over UPLINK network.
  lxc exec micro01 -- ip link add UPLINK type bridge

  join_session init micro01 micro02 micro03
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  # Check MicroCloud observed the left over UPLINK network.
  lxc exec micro01 -- grep -q "Warning: System \"micro01\" is ineligible for distributed networking. Make sure there aren't any conflicting networks from previous installations" out

  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 1 1
    validate_system_microceph "${m}" 1 disk2
  done
}

_test_case() {
    # Number of systems to use in the test.
    num_systems="${1}"

    # Number of available disks per system.
    num_disks="${2}"

    # Number of available interfaces per system.
    num_ifaces="${3}"

    shift 3
    skip_services="${*}"

    # Refuse to create local storage, even if we have enough disks and peers.
    force_no_zfs="$(echo "${skip_services}" | grep -Fwo zfs || true)"

    # Refuse to create ceph storage, even if we have enough disks and peers.
    force_no_ceph="$(echo "${skip_services}" | grep -Fwo ceph || true)"

    # Refuse to create ovn network, even if we have enough interfaces and peers.
    force_no_ovn="$(echo "${skip_services}" | grep -Fwo ovn || true)"

    expected_zfs_disk=""
    expected_ceph_disks=0
    expected_cephfs=0
    expected_ovn_iface=""

    unset_interactive_vars

    reset_systems "${num_systems}" "${num_disks}" "${num_ifaces}"
    printf "Creating a MicroCloud with %d systems, %d disks, %d extra interfaces" "${num_systems}" "${num_disks}" "${num_ifaces}"
    if [ -n "${force_no_zfs}" ]; then
      printf ", and skipping zfs setup"
    fi

    if [ -n "${force_no_ceph}" ]; then
      printf ", and skipping ceph setup"
    fi

    if [ -n "${force_no_ovn}" ]; then
      printf ", and skipping ovn setup"
    fi

    printf "\n"

    microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

    export MULTI_NODE="yes"
    export LOOKUP_IFACE="enp5s0" # filter string for the lookup interface table.
    export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
    export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
    export OVN_UNDERLAY_NETWORK="no"

    export EXPECT_PEERS="$((num_systems - 1))"

    if [ "${num_disks}" -gt 0 ] ; then
      if [ -z "${force_no_zfs}" ]; then
        export SETUP_ZFS="yes"
        export ZFS_FILTER="disk1"
        export ZFS_WIPE="yes"
        expected_zfs_disk="disk1"
      else
        export SETUP_ZFS="no"
      fi
    fi

    if [ "${num_disks}" -gt 0 ] ; then
      # If we only have one disk and we used it for ZFS, there should be no prompt.
      if [ "${num_disks}" = 1 ] && [ -z "${force_no_zfs}" ] ; then
        echo "Insufficient disks to test Remote storage"
      elif [ -z "${force_no_ceph}" ]; then
        export SETUP_CEPH="yes"
        export SETUP_CEPHFS="yes"
        export CEPH_WIPE="yes"
        export CEPH_ENCRYPT="no"
        expected_ceph_disks="${num_disks}"
        if [ -n "${expected_zfs_disk}" ]; then
          expected_ceph_disks="$((num_disks - 1))"
        fi

        if [ "${expected_ceph_disks}" -gt 0 ]; then
          expected_cephfs=1
        fi

        if [ "${num_systems}" -lt 3 ]; then
          export CEPH_RETRY_HA="no"
        fi
      else
        export SETUP_CEPH="no"
      fi
    fi

    if [ "${num_ifaces}" -gt 0 ] ; then
      if [ -z "${force_no_ovn}" ] ; then
        export SETUP_OVN_EXPLICIT="yes"

        # Always pick the first available interface.
        export OVN_FILTER="enp6s0"
        export IPV4_SUBNET="10.1.123.1/24"
        export IPV4_START="10.1.123.100"
        export IPV4_END="10.1.123.254"
        export IPV6_SUBNET="fd42:1:1234:1234::1/64"
        export DNS_ADDRESSES="10.1.123.1,8.8.8.8"

        expected_ovn_iface="enp6s0"
      else
        export SETUP_OVN_EXPLICIT="no"
      fi
    else
      export OVN_WARNING="yes"
    fi

    join_systems=""
    for i in $(seq -f "%02g" 2 "${num_systems}") ; do
      join_systems+=" micro${i}"
    done

    # shellcheck disable=SC2086
    join_session init micro01 $join_systems
    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    for i in $(seq -f "%02g" 1 "${num_systems}") ; do
      name="micro${i}"

      if [ -n "${expected_ovn_iface}" ]; then
        validate_system_lxd "${name}" "${num_systems}" "${expected_zfs_disk}" "${expected_ceph_disks}" "${expected_cephfs}" "${expected_ovn_iface}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
      else
        validate_system_lxd "${name}" "${num_systems}" "${expected_zfs_disk}" "${expected_ceph_disks}" "${expected_cephfs}" "${expected_ovn_iface}"
      fi

      start_disk=1
      if [ -n "${expected_zfs_disk}" ]; then
        start_disk=2
      fi

      ceph_disks=""

      if [ "${expected_ceph_disks}" -gt 0 ]; then
        for j in $(seq "${start_disk}" "${num_disks}"); do
          ceph_disks="$(echo "${ceph_disks} disk${j}" | sed -e 's/^ //')"
        done
      fi

      validate_system_microceph "${name}" "${expected_cephfs}" "${ceph_disks}"
      validate_system_microovn "${name}"
    done
}


test_interactive_combinations() {
  # Test with 2 systems, no disks, no interfaces.
  _test_case 2 0 0

  # Test with 2 systems, 1 disk, no interfaces, and each combination of skipping ZFS, Ceph.
  _test_case 2 1 0
  _test_case 2 1 0 "zfs"
  _test_case 2 1 0 "ceph"
  _test_case 2 1 0 "zfs" "ceph"

  # Test with 2 systems, 0 disks, 1 interface, and each combination of skipping OVN.
  _test_case 2 0 1
  _test_case 2 0 1 "ovn"

  # Test with 2 systems, 1 disks, 1 interface, and each combination of skipping ZFS, Ceph, OVN.
  _test_case 2 1 1
  _test_case 2 1 1 "zfs"
  _test_case 2 1 1 "ceph"
  _test_case 2 1 1 "zfs" "ceph"
  _test_case 2 1 1 "ovn"
  _test_case 2 1 1 "zfs" "ovn"
  _test_case 2 1 1 "ceph" "ovn"
  _test_case 2 1 1 "zfs" "ceph" "ovn"

  # Test with 2 systems, 2 disks, 1 interface, and each combination of skipping ZFS, Ceph, OVN.
  _test_case 2 2 1
  _test_case 2 2 1 "zfs"
  _test_case 2 2 1 "ceph"
  _test_case 2 2 1 "zfs" "ceph"
  _test_case 2 2 1 "ovn"
  _test_case 2 2 1 "zfs" "ovn"
  _test_case 2 2 1 "ceph" "ovn"
  _test_case 2 2 1 "zfs" "ceph" "ovn"

  # Test with 2 systems, 3 disks, 1 interface, and each combination of skipping ZFS, Ceph, OVN.
  _test_case 2 3 1
  _test_case 2 3 1 "zfs"
  _test_case 2 3 1 "ceph"
  _test_case 2 3 1 "zfs" "ceph"
  _test_case 2 3 1 "ovn"
  _test_case 2 3 1 "zfs" "ovn"
  _test_case 2 3 1 "ceph" "ovn"
  _test_case 2 3 1 "zfs" "ceph" "ovn"

  # Test with 3 systems, with and without disks & interfaces.
  _test_case 3 0 0
  _test_case 3 2 2

  # Test with 4 systems, with and without disks & interfaces.
  _test_case 4 0 0
  _test_case 4 2 2
}

test_service_mismatch() {
  unset_interactive_vars
  # Selects all available systems, adds 1 local disk per system, skips ceph and ovn setup.
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="no"
  export SETUP_OVN_EXPLICIT="no"

  # Restore the snapshots from the previous test.
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"

  # Install microceph and microovn on the first machine only.
  for m in micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph
    lxc exec "${m}" -- snap disable microovn
    lxc exec "${m}" -- snap restart microcloud
  done

  # Init should fail to find the other systems as they don't have the same services.
  # The error is reported on the joining side.
  echo "Peers with missing services cannot join"
  ! join_session init micro01 micro02 micro03 || false

  # Ensure the joiners exited due to missing services.
  # The initiator exits automatically after the session timeout.
  lxc exec micro02 -- tail -1 out | grep "Rejecting peer \"micro02\" due to missing services" -q
  lxc exec micro03 -- tail -1 out | grep "Rejecting peer \"micro03\" due to missing services" -q

  # Install the remaining services on the other systems.
  lxc exec micro02 -- snap enable microceph
  lxc exec micro02 -- snap enable microovn
  lxc exec micro02 -- snap restart microcloud
  lxc exec micro03 -- snap enable microceph
  lxc exec micro03 -- snap enable microovn
  lxc exec micro03 -- snap restart microcloud

  # Init should now work.
  echo "Creating a MicroCloud with MicroCeph and MicroOVN, but without their LXD devices"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  reset_systems 1 3 1

  lxc exec micro01 -- sh -c "echo 1 > /proc/sys/net/ipv6/conf/enp5s0/disable_ipv6"
  retry lxc exec micro01 -- snap refresh microceph --channel reef/stable

  ! join_session init micro01 || false
  lxc exec micro01 -- tail -1 out | grep -q "The installed version of MicroCeph is not supported"

  lxc exec micro01 -- rm -rf out
  ! lxc exec micro01 --env "TEST_CONSOLE=0" -- sh -c "microcloud join 2> out" || false
  lxc exec micro01 -- tail -1 out | grep -q "The installed version of MicroCeph is not supported"

  # Try to set up a LXD-only MicroCloud while some systems have other services present.
  reset_systems 3 3 1

  # Run all services on the other systems only.
  lxc exec micro01 -- snap disable microceph || true
  lxc exec micro01 -- snap disable microovn || true
  lxc exec micro01 -- snap restart microcloud

  SKIP_SERVICE="yes"
  unset SETUP_CEPH SETUP_OVN_EXPLICIT
  # Init from the minimal system should work, but not set up any services it doesn't have.
  echo "Creating a MicroCloud without setting up MicroOVN and MicroCeph on peers"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  for m in micro02 micro03 ; do
   lxc exec ${m} -- microceph cluster list 2>&1 | grep "Error: Database is not yet initialized" -q
   lxc exec ${m} -- microovn cluster list  2>&1 | grep "Error: Database is not yet initialized" -q
  done
}

test_disk_mismatch() {
  reset_systems 3 3 1

  # Setup micro04 with only 1 disk for ZFS.
  reset_system micro04 1 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with fully remote ceph on one node"
  unset_interactive_vars
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=3
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_WIPE="yes"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN_EXPLICIT="no"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  join_session init micro01 micro02 micro03 micro04
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 6 1
    validate_system_microovn "${m}"
  done

  for m in micro01 micro02 micro03 ; do
    validate_system_microceph "${m}" 1 disk2 disk3
  done

  validate_system_microceph "micro04"
}

# services_validator: A basic validator of 3 systems with typical expected inputs.
# An optional pool name can be provided which has to be present in the default profile's root device.
services_validator() {
  profile_pool="${1:-}"

  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8 "${profile_pool}"
    validate_system_microceph "${m}" 1 disk2
    validate_system_microovn "${m}"
  done
}

# bootstrap_microceph: Wrapper for bootstrapping MicroCeph on the given system.
bootstrap_microceph() {
  lxc exec "${1}" -- microceph cluster bootstrap

  # Wait until the services are deployed.
  # This is to ensure the test suite doesn't try to access the MicroCeph's socket too quickly after
  # bootstrapping to prevent running into timeouts.
  # See https://github.com/canonical/microceph/issues/473.
  retries=0
  while true; do
    if [ "${retries}" -gt 60 ]; then
      echo "Retries exceeded whilst waiting for MicroCeph on ${1} to become available"
      exit 1
    fi

    if lxc exec "${1}" -- microceph status | grep -q "Services: mds, mgr, mon"; then
      break
    fi

    sleep 1
    retries="$((retries+1))"
  done;
}

test_reuse_cluster() {
  unset_interactive_vars

  # Set the default config for interactive setup.
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export OVN_UNDERLAY_NETWORK="no"

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service"
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="yes"
  bootstrap_microceph micro02
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service on the local node"
  bootstrap_microceph micro01
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing MicroCeph and MicroOVN"
  export REUSE_EXISTING_COUNT=2
  export REUSE_EXISTING="yes"
  bootstrap_microceph micro02
  lxc exec micro02 -- microovn cluster bootstrap
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing MicroCeph and MicroOVN on different nodes"
  bootstrap_microceph micro02
  lxc exec micro03 -- microovn cluster bootstrap
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service with multiple nodes from this cluster"
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="yes"
  bootstrap_microceph micro02
  token="$(lxc exec micro02 -- microceph cluster add micro01)"
  lxc exec micro01 -- microceph cluster join "${token}"
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing existing service with all nodes from this cluster"
  bootstrap_microceph micro02
  token="$(lxc exec micro02 -- microceph cluster add micro01)"
  lxc exec micro01 -- microceph cluster join "${token}"
  token="$(lxc exec micro02 -- microceph cluster add micro03)"
  lxc exec micro03 -- microceph cluster join "${token}"
  join_session init micro01 micro02 micro03
  services_validator

  reset_systems 4 3 3
  echo "Create a MicroCloud that re-uses an existing existing service with foreign cluster members"
  lxc exec micro04 -- snap disable microcloud
  bootstrap_microceph micro02
  token="$(lxc exec micro02 -- microceph cluster add micro04)"
  lxc exec micro04 -- microceph cluster join "${token}"
  join_session init micro01 micro02 micro03
  services_validator
  validate_system_microceph micro04 1

  reset_systems 3 3 3
  echo "Fail to create a MicroCloud due to conflicting existing services"
  bootstrap_microceph micro02
  bootstrap_microceph micro03
  ! join_session init micro01 micro02 micro03 || false
  lxc exec micro01 -- tail -1 out | grep "Some systems are already part of different MicroCeph clusters. Aborting initialization" -q

  reset_systems 3 2 3
  echo "Create a MicroCloud that re-uses an existing MicroCeph with disks already setup to configure distributed storage"
  unset_interactive_vars

  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SKIP_CEPH_DISKS="yes"
  export SETUP_CEPHFS="yes"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export OVN_UNDERLAY_NETWORK="no"

  bootstrap_microceph micro01
  for m in micro02 micro03; do
    token="$(lxc exec micro01 -- microceph cluster add "${m}")"
    lxc exec "${m}" -- microceph cluster join "${token}"
  done

  for m in micro01 micro02 micro03; do
    lxc exec "${m}" -- microceph disk add /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
  done

  join_session init micro01 micro02 micro03
  services_validator
}

test_remove_cluster() {
  unset_interactive_vars

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  # Set the default config for interactive setup.
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_ENCRYPT="no"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export OVN_UNDERLAY_NETWORK="no"

  reset_systems 3 3 3
  echo "Fail to remove member from MicroCeph and LXD until OSDs are removed"
  join_session init micro01 micro02 micro03

  # Wait for roles to refresh from the next heartbeat.
  for i in $(seq 1 40) ; do
    if lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster list | grep -q PENDING ; then
      sleep 1
    else
      break
    fi
  done

  ! lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro02 || false

  # No systems are removed, because LXD is attempted first, and fails to be removed cleanly.
  for s in "microcloud" "microovn" "microceph" "lxc"; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done

  lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro02 --force

  for s in "microcloud" "microovn" "microceph" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    ! lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02" || false
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done

  # Fail to shrink the cluster down to 1 member, because of MicroCeph's monmap restriction.
  ! lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro03 --force || false
  lxc exec micro01 --env "TEST_CONSOLE=0" -- microceph.ceph mon remove micro03
  lxc exec micro01 --env "TEST_CONSOLE=0" -- microceph cluster sql "delete from services where member_id = (select id from core_cluster_members where name='micro03') and service='mon'"
  lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro03 --force

  for s in "microcloud" "microovn" "microceph" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    ! lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02" || false
    ! lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03" || false
  done

  # Ensure ceph & ovn still work.
  lxc exec micro01 -- microceph.ceph osd ls
  lxc exec micro01 -- microovn.ovn-sbctl find Encap

  # With 3 or fewer systems, MicroCeph requires --force to be used to remove cluster members. We tested that above so ignore MicroCeph for the rest of the test.
  unset SETUP_CEPH
  unset CEPH_CLUSTER_NETWORK
  unset CEPH_PUBLIC_NETWORK
  export SKIP_SERVICE="yes"

  # LXD will require --force if we have storage volumes, so don't set those up.
  export SETUP_ZFS="no"
  unset ZFS_FILTER
  unset ZFS_WIPE

  reset_systems 3 3 3
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  echo "Create a MicroCloud and remove a node from all services"
  join_session init micro01 micro02 micro03

  # Wait for roles to refresh from the next heartbeat.
  for i in $(seq 1 40) ; do
    if lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster list | grep -q PENDING ; then
      sleep 1
    else
      break
    fi
  done

  lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro02

  for s in "microcloud" "microovn" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    ! lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02" || false
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done

  reset_systems 3 3 3
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  echo "Create a MicroCloud and remove a node from all services, but manually remove it from the MicroCloud daemon first"
  join_session init micro01 micro02 micro03

  # Wait for roles to refresh from the next heartbeat.
  for i in $(seq 1 40) ; do
    if lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster list | grep -q PENDING ; then
      sleep 1
    else
      break
    fi
  done

  lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster remove micro02

  ! lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster list | grep -q "micro02" || false
  for s in "microovn" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done

  lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove micro02

  for s in "microcloud" "microovn" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    ! lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02" || false
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done

  reset_systems 3 3 3
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  echo "Create a MicroCloud and fail to remove a non-existent member"
  join_session init micro01 micro02 micro03

  for i in $(seq 1 40) ; do
    if lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud cluster list | grep -q PENDING ; then
      sleep 1
    else
      break
    fi
  done

  ! lxc exec micro01 --env "TEST_CONSOLE=0" -- microcloud remove abcd || false

  for s in "microcloud" "microovn" "lxc" ; do
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro01"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro02"
    lxc exec micro01 --env "TEST_CONSOLE=0" -- ${s} cluster list | grep -q "micro03"
  done
}

test_add_services() {
  unset_interactive_vars
  # Set the default config for interactive setup.

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.0.2"
  ceph_public_subnet_iface="enp8s0"
  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_ENCRYPT="no"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_public_subnet_prefix}.0/24"
  export OVN_UNDERLAY_NETWORK="no"

  reset_systems 3 3 3
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
  echo Add MicroCeph to MicroCloud that was set up without it, and setup remote storage without updating the profile.
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  unset SETUP_CEPH
  export SKIP_SERVICE="yes"
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- snap enable microceph
  lxc exec micro01 -- snap restart microcloud
  export SETUP_CEPH="yes"
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  unset SETUP_ZFS
  unset SETUP_OVN_EXPLICIT
  export REPLACE_PROFILE="no"
  microcloud_interactive "service add" micro01
  # The initial cluster got setup with only local storage.
  # Due to REPLACE_PROFILE="no" the default profile's device doesn't get replaced.
  services_validator local

  reset_systems 3 3 3
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
  echo Add MicroCeph to MicroCloud that was set up without it, and setup remote storage.
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  unset SETUP_CEPH
  unset REPLACE_PROFILE
  unset SKIP_LOOKUP
  export MULTI_NODE="yes"
  export SKIP_SERVICE="yes"
  export SETUP_ZFS="yes"
  export SETUP_OVN_EXPLICIT="yes"
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- snap enable microceph
  lxc exec micro01 -- snap restart microcloud
  export SETUP_CEPH="yes"
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  unset SETUP_ZFS
  unset SETUP_OVN_EXPLICIT
  export REPLACE_PROFILE="yes"
  microcloud_interactive "service add" micro01
  services_validator

  reset_systems 3 3 3
  set_cluster_subnet 3 "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"

  echo Add MicroOVN to MicroCloud that was set up without it, and setup ovn network
  lxc exec micro01 -- snap disable microovn
  lxc exec micro01 -- snap restart microcloud
  export MULTI_NODE="yes"
  export SETUP_ZFS="yes"
  unset SKIP_LOOKUP
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- snap enable microovn
  lxc exec micro01 -- snap restart microcloud
  export SETUP_OVN_EXPLICIT="yes"
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  unset SETUP_ZFS
  unset SETUP_CEPH
  microcloud_interactive "service add" micro01
  services_validator

  reset_systems 3 3 3
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"

  echo Add both MicroOVN and MicroCeph to a MicroCloud that was set up without it
  lxc exec micro01 -- snap disable microovn
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  export MULTI_NODE="yes"
  export SETUP_ZFS="yes"
  unset SKIP_LOOKUP
  unset SETUP_OVN_EXPLICIT
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- snap enable microovn
  lxc exec micro01 -- snap enable microceph
  lxc exec micro01 -- snap restart microcloud
  export SETUP_OVN_EXPLICIT="yes"
  export SETUP_CEPH="yes"
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  unset SETUP_ZFS
  microcloud_interactive "service add" micro01
  services_validator

  reset_systems 3 3 3
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"

  echo Reuse a MicroCeph that was set up on one node of the MicroCloud
  lxc exec micro01 -- snap disable microceph
  lxc exec micro01 -- snap restart microcloud
  bootstrap_microceph micro02
  export MULTI_NODE="yes"
  export SETUP_ZFS="yes"
  unset SETUP_CEPH
  unset SKIP_LOOKUP
  join_session init micro01 micro02 micro03
  lxc exec micro01 -- snap enable microceph
  lxc exec micro01 -- snap restart microcloud
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="yes"
  export SETUP_CEPH="yes"
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  unset SETUP_ZFS
  unset SETUP_OVN_EXPLICIT
  unset CEPH_CLUSTER_NETWORK
  unset CEPH_PUBLIC_NETWORK
  microcloud_interactive "service add" micro01
  services_validator

  reset_systems 3 3 3
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"

  echo Fail to add any services if they have been set up
  export MULTI_NODE="yes"
  export SETUP_ZFS="yes"
  export SETUP_OVN_EXPLICIT="yes"
  unset REUSE_EXISTING
  unset REUSE_EXISTING_COUNT
  unset SKIP_LOOKUP
  unset SKIP_SERVICE
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export CEPH_PUBLIC_NETWORK="${ceph_public_subnet_prefix}.0/24"
  join_session init micro01 micro02 micro03
  export SKIP_LOOKUP=1
  unset MULTI_NODE
  ! microcloud_interactive "service add" micro01 || true
}

test_non_ha() {
  unset_interactive_vars
  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  export MULTI_NODE="yes"
  export LOOKUP_IFACE="enp5s0"
  export EXPECT_PEERS=1
  export SETUP_ZFS="no"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk1"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_ENCRYPT="no"
  export CEPH_RETRY_HA="no"
  export SETUP_OVN_EXPLICIT="yes"
  export OVN_UNDERLAY_NETWORK="no"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"

  reset_systems 2 1 3
  echo "Creating a MicroCloud with 2 systems and only Ceph storage"
  join_session init micro01 micro02
  for m in micro01 micro02 ; do
    validate_system_lxd ${m} 2 "" 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m} 1 disk1
    validate_system_microovn ${m}
  done

  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  unset SETUP_CEPH
  reset_systems 2 1 3
  echo "Creating a MicroCloud with 2 systems and only ZFS storage"
  join_session init micro01 micro02
  for m in micro01 micro02 ; do
    validate_system_lxd ${m} 2 "disk1" 0 0 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m}
    validate_system_microovn ${m}
  done

  export SETUP_CEPH="yes"
  export CEPH_FILTER="lxd_disk2"
  reset_systems 2 2 3
  echo "Creating a MicroCloud with 2 systems and all storage & networks"
  join_session init micro01 micro02
  for m in micro01 micro02 ; do
    validate_system_lxd ${m} 2 "disk1" 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m} 1 "disk2"
    validate_system_microovn ${m}
  done

  reset_systems 3 2 2
  for m in micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microcloud
  done

  export MULTI_NODE="no"
  export SKIP_LOOKUP=1
  echo "Creating a MicroCloud with 1 system, and grow it to 3 with all storage & networks"
  join_session init micro01
  validate_system_lxd "micro01" 1 "disk1" 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
  validate_system_microceph "micro01" 1 "disk2"
  validate_system_microovn "micro01"

  for m in micro02 micro03 ; do
    lxc exec "${m}" -- snap enable microcloud
    lxc exec "${m}" -- snap start microcloud
  done

  unset SKIP_LOOKUP
  unset MULTI_NODE
  unset CEPH_CLUSTER_NETWORK
  unset CEPH_PUBLIC_NETWORK
  unset CEPH_RETRY_HA
  unset SETUP_OVN_EXPLICIT
  unset IPV4_SUBNET IPV4_START IPV4_END DNS_ADDRESSES IPV6_SUBNET
  unset SETUP_CEPHFS
  export EXPECT_PEERS=2
  export SETUP_OVN_IMPLICIT="yes"
  join_session add micro01 micro02 micro03
  for m in micro1 micro2 micro3 ; do
    validate_system_lxd "micro01" 3 "disk1" 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph "micro01" 1 "disk2"
    validate_system_microovn "micro01"
  done

  reset_systems 2 3 3
  echo "Creating a MicroCloud with 1 system and growing it to 3, using preseed"
  addr=$(lxc ls micro01 -f csv -c4 | awk '/enp5s0/ {print $1}')
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8
ceph:
  cephfs: true
EOF
  )"

  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  validate_system_lxd "micro01" 1 "" 2 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
  validate_system_microceph "micro01" 1 "disk2" "disk3"
  validate_system_microovn "micro01"

  addr=$(lxc ls micro01 -f csv -c4 | awk '/enp5s0/ {print $1}')
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro02
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 ; do
    validate_system_lxd ${m} 2 "" 2 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m} 1 "disk2" "disk3"
    validate_system_microovn ${m}
  done

  reset_systems 2 3 3
  echo "Creating a MicroCloud with 2 systems with Ceph storage using preseed"
  addr=$(lxc ls micro01 -f csv -c4 | awk '/enp5s0/ {print $1}')
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
- name: micro02
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk3
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8
ceph:
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  for m in micro01 micro02 ; do
    validate_system_lxd ${m} 2 "" 2 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m} 1 "disk2" "disk3"
    validate_system_microovn ${m}
  done
}
