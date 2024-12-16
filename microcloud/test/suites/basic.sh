#!/bin/bash

test_interactive() {
  reset_systems 3 3 1

  echo "Creating a MicroCloud with all services but no devices"
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="no"
  export SETUP_CEPH="no"
  export SETUP_OVN="no"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
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

  echo "Creating a MicroCloud with ZFS storage"
  export SKIP_SERVICE="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  unset SETUP_CEPH SETUP_OVN
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems and install microceph.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

  echo "Creating a MicroCloud with ZFS and Ceph storage"
  export SETUP_CEPH="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 1
    validate_system_microceph "${m}" disk2
  done

  # Reset the systems and install microovn.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap restart microcloud
  done

  echo "Creating a MicroCloud with ZFS storage and OVN network"
  unset SETUP_CEPH CEPH_FILTER CEPH_WIPE

  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 0 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph.
  reset_systems 3 3 1

  echo "Creating a MicroCloud with ZFS and Ceph storage, and OVN network"
  unset SKIP_SERVICE
  export SETUP_CEPH="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" disk2
    validate_system_microovn "${m}"
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

    export LOOKUP_IFACE="enp5s0" # filter string for the lookup interface table.
    export LIMIT_SUBNET="yes" # (yes/no) input for limiting lookup of systems to the above subnet.

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

    if [ "${num_disks}" -gt 0 ] && [ "${num_systems}" -gt 2 ]; then
      # If we only have one disk and we used it for ZFS, there should be no prompt.
      if [ "${num_disks}" = 1 ] && [ -z "${force_no_zfs}" ] ; then
        echo "Insufficient disks to test Remote storage"
      elif [ -z "${force_no_ceph}" ]; then
        export SETUP_CEPH="yes"
        export CEPH_WIPE="yes"
        expected_ceph_disks="${num_disks}"
        if [ -n "${expected_zfs_disk}" ]; then
          expected_ceph_disks="$((num_disks - 1))"
        fi
      else
        export SETUP_CEPH="no"
      fi
    fi

    if [ "${num_ifaces}" -gt 0 ] ; then
      if [ -z "${force_no_ovn}" ] ; then
        export SETUP_OVN="yes"

        # Always pick the first available interface.
        export OVN_FILTER="enp6s0"
        export IPV4_SUBNET="10.1.123.1/24"
        export IPV4_START="10.1.123.100"
        export IPV4_END="10.1.123.254"
        export IPV6_SUBNET="fd42:1:1234:1234::1/64"

        expected_ovn_iface="enp6s0"
      else
        export SETUP_OVN="no"
      fi
    fi

    microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    for i in $(seq -f "%02g" 1 "${num_systems}") ; do
      name="micro${i}"

      if [ -n "${expected_ovn_iface}" ]; then
        validate_system_lxd "${name}" "${num_systems}" "${expected_zfs_disk}" "${expected_ceph_disks}" "${expected_ovn_iface}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
      else
        validate_system_lxd "${name}" "${num_systems}" "${expected_zfs_disk}" "${expected_ceph_disks}" "${expected_ovn_iface}"
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

      validate_system_microceph "${name}"  "${ceph_disks}"
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
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="no"
  export SETUP_OVN="no"

  # Restore the snapshots from the previous test.
  reset_systems 3 3 1

  # Install microceph and microovn on the first machine only.
  for m in micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph
    lxc exec "${m}" -- snap disable microovn
    lxc exec "${m}" -- snap restart microcloud
  done

  # Init should fail to find the other systems as they don't have the same services.
  # The error is reported on the joining side.
  echo "Peers with missing services cannot join"
  ! microcloud_interactive | lxc exec micro01 -- sh -c "timeout -k 5 30 microcloud init > out" || false

  # Ensure we exited while still looking for servers, and found none.
  lxc exec micro01 -- tail -1 out | grep "Scanning for eligible servers" -q

  # Install the remaining services on the other systems.
  lxc exec micro02 -- snap enable microceph
  lxc exec micro02 -- snap enable microovn
  lxc exec micro03 -- snap enable microceph
  lxc exec micro03 -- snap enable microovn

  # Init should now work.
  echo "Creating a MicroCloud with MicroCeph and MicroOVN, but without their LXD devices"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  # Try to set up a LXD-only MicroCloud while some systems have other services present.
  reset_systems 3 3 1

  # Run all services on the other systems only.
  lxc exec micro01 -- snap disable microceph || true
  lxc exec micro01 -- snap disable microovn || true
  lxc exec micro01 -- snap restart microcloud

  SKIP_SERVICE="yes"
  unset SETUP_CEPH SETUP_OVN
  # Init from the minimal system should work, but not set up any services it doesn't have.
  echo "Creating a MicroCloud without setting up MicroOVN and MicroCeph on peers"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  for m in micro02 micro03 ; do
    # reef uses a different microcluster version with a different error message.
    if lxc exec ${m} -- snap list microceph | grep -q "reef" ; then
     lxc exec ${m} -- sh -c "microceph cluster list" 2>&1 | grep "Error: Database is not yet initialized" -q
    else
     lxc exec ${m} -- sh -c "microceph cluster list" 2>&1 | grep "Error: Daemon not yet initialized" -q
    fi

   lxc exec ${m} -- sh -c "microovn cluster list" 2>&1 | grep "Error: Daemon not yet initialized" -q
  done
}

test_disk_mismatch() {
  reset_systems 3 3 1

  # Setup micro04 with only 1 disk for ZFS.
  reset_system micro04 1 1

  echo "Creating a MicroCloud with fully remote ceph on one node"
  unset_interactive_vars
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=3
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export CEPH_WARNING="yes"
  export CEPH_WIPE="yes"
  export SETUP_OVN="no"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 micro04 ; do
    validate_system_lxd "${m}" 4 disk1 6
    validate_system_microovn "${m}"
  done

  for m in micro01 micro02 micro03 ; do
    validate_system_microceph "${m}" disk2 disk3
  done

  validate_system_microceph "micro04"
}

# Test automatic setup with a variety of devices.
test_auto() {
  reset_systems 2 0 0

  lxc exec micro02 -- snap stop microcloud

  echo MicroCloud auto setup without any peers.
  ! lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out 2>&1" || false
  lxc exec micro01 -- tail -1 out | grep -q "Error: Found no available systems"

  lxc exec micro02 -- snap start microcloud

  echo Auto-create a MicroCloud with 2 systems with no disks/interfaces.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Supress the first message from LXD.
    lxc exec ${m} -- lxc list > /dev/null 2>&1 || true

    # Ensure we created no storage devices.
    [ "$(lxc exec ${m} -- lxc storage ls -f csv | wc -l)" = "0" ]
  done

  reset_systems 2 0 1

  echo Auto-create a MicroCloud with 2 systems with 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false

    # Ensure we created no storage devices.
    [ "$(lxc exec ${m} -- lxc storage ls -f csv | wc -l)" = "0" ]
  done


  reset_systems 2 3 1

  echo Auto-create a MicroCloud with 2 systems with 3 disks and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2 disk1
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false

    # Ensure we created no ceph storage devices.
    ! lxc exec ${m} -- lxc storage ls -f csv | grep -q "^remote,ceph" || false
  done

  reset_systems 3 0 0

  echo Auto-create a MicroCloud with 3 systems with no disks/interfaces.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Supress the first message from LXD.
    lxc exec ${m} -- lxc list > /dev/null 2>&1 || true

    # Ensure we created no storage devices.
    [ "$(lxc exec ${m} -- lxc storage ls -f csv | wc -l)" = "0" ]
  done

  reset_systems 3 0 1

  echo Auto-create a MicroCloud with 3 systems with 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false

    # Ensure we created no storage devices.
    [ "$(lxc exec ${m} -- lxc storage ls -f csv | wc -l)" = "0" ]
  done

  reset_systems 3 1 1

  echo Auto-create a MicroCloud with 3 systems with 1 disk and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03; do
    validate_system_lxd "${m}" 3 "" 1
    validate_system_microceph "${m}" disk1
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false

    # Ensure we created no zfs storage devices.
    ! lxc exec ${m} -- lxc storage ls -f csv | grep -q "^local,zfs" || false
  done

  reset_systems 3 3 1

  echo Auto-create a MicroCloud with 3 systems with 3 disks and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 2
    validate_system_microceph "${m}" disk2 disk3
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false
  done
}
