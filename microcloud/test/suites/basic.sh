test_interactive() {
  reset_systems 3 3 1

  echo "Creating a MicroCloud with all services but no devices"
  LOOKUP_IFACE="enp5s0"
  LIMIT_SUBNET="yes"
  EXPECT_PEERS=2
  SETUP_ZFS="no"
  SETUP_CEPH="no"
  SETUP_OVN="no"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd ${m} 3
    validate_system_microceph ${m}
    validate_system_microovn ${m}
  done

  # Reset the systems with just LXD.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec ${m} -- sh -c "snap disable microceph || true"
    lxc exec ${m} -- sh -c "snap disable microovn || true"
    lxc exec ${m} -- sh -c "snap restart microcloud"
  done

  echo "Creating a MicroCloud with ZFS storage"
  SKIP_SERVICE=yes
  SETUP_ZFS="yes"
  ZFS_FILTER="lxd_disk1"
  ZFS_WIPE="yes"
  unset SETUP_CEPH SETUP_OVN
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems and install microceph.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec ${m} -- sh -c "snap disable microovn || true"
    lxc exec ${m} -- sh -c "snap restart microcloud"
  done

  echo "Creating a MicroCloud with ZFS and Ceph storage"
  SETUP_CEPH="yes"
  CEPH_FILTER="lxd_disk2"
  CEPH_WIPE="yes"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 1
    validate_system_microceph "${m}" disk2
  done

  # Reset the systems and install microovn.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec ${m} -- sh -c "snap disable microceph || true"
    lxc exec ${m} -- sh -c "snap restart microcloud"
  done

  echo "Creating a MicroCloud with ZFS storage and OVN network"
  unset SETUP_CEPH CEPH_FILTER CEPH_WIPE

  SETUP_OVN="yes"
  OVN_FILTER="enp6s0"
  IPV4_SUBNET="10.1.123.1/24"
  IPV4_START="10.1.123.100"
  IPV4_END="10.1.123.254"
  IPV6_SUBNET="fd42:1:1234:1234::1/64"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 0 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph.
  reset_systems 3 3 1

  echo "Creating a MicroCloud with ZFS and Ceph storage, and OVN network"
  unset SKIP_SERVICE
  SETUP_CEPH="yes"
  CEPH_FILTER="lxd_disk2"
  CEPH_WIPE="yes"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
    validate_system_microceph "${m}" disk2
    validate_system_microovn "${m}"
  done
}

test_case() {
    # Number of systems to use in the test.
    local num_systems num_disks num_ifaces skip_services
    num_systems=$1

    # Number of available disks per system.
    num_disks=$2

    # Number of available interfaces per system.
    num_ifaces=$3

    skip_services=${4:-}

    # Refuse to create local storage, even if we have enough disks and peers.
    force_no_zfs="$(echo ${@} | grep -o zfs || true)"

    # Refuse to create ceph storage, even if we have enough disks and peers.
    force_no_ceph="$(echo ${@} | grep -o ceph || true)"

    # Refuse to create ovn network, even if we have enough interfaces and peers.
    force_no_ovn="$(echo ${@} | grep -o ovn || true)"


    expected_zfs_disk=""
    expected_ceph_disks=0
    expected_ovn_iface=""

    unset_interactive_vars

    reset_systems ${num_systems} ${num_disks} ${num_ifaces}
    printf "Creating a MicroCloud with ${num_systems} systems, ${num_disks} disks, ${num_ifaces} extra interfaces"
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

    LOOKUP_IFACE=enp5s0 # filter string for the lookup interface table.
    LIMIT_SUBNET=yes # (yes/no) input for limiting lookup of systems to the above subnet.

    EXPECT_PEERS=$((num_systems - 1))

    if [ ${num_disks} -gt 0 ] ; then
      if [ -z "${force_no_zfs}" ]; then
        SETUP_ZFS=yes
        ZFS_FILTER=disk1
        ZFS_WIPE=yes
        expected_zfs_disk="disk1"
      else
        SETUP_ZFS=no
      fi
    fi

    if [ ${num_disks} -gt 0 ] && [ ${num_systems} -ge 3 ] ; then
      # If we only have one disk and we used it for ZFS, there should be no prompt.
      if [ ${num_disks} = 1 ] && [ -z "${force_no_zfs}" ] ; then
        insufficient_disks_warning=1
      elif [ -z "${force_no_ceph}" ]; then
        SETUP_CEPH=yes
        CEPH_WIPE=yes
        expected_ceph_disks=${num_disks}
        if [ -n "${expected_zfs_disk}" ]; then
          expected_ceph_disks=$((num_disks - 1))
        fi
      else
        SETUP_CEPH=no
      fi
    fi

    if [ ${num_ifaces} -gt 0 ] && [ ${num_systems} -ge 3 ] ; then
      if [ -z "${force_no_ovn}" ] ; then
        SETUP_OVN=yes

        # Always pick the first available interface.
        OVN_FILTER=enp6s0
        IPV4_SUBNET="10.1.123.1/24"
        IPV4_START="10.1.123.100"
        IPV4_END="10.1.123.254"
        IPV6_SUBNET="fd42:1:1234:1234::1/64"

        expected_ovn_iface="enp6s0"
      else
        SETUP_OVN=no
      fi
    fi

    microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
    lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
    for i in $(seq 1 ${num_systems}) ; do
      name=$(printf "micro%02d" $i)

      if [ -n "${expected_ovn_iface}" ]; then
        validate_system_lxd "${name}" ${num_systems} "${expected_zfs_disk}" ${expected_ceph_disks} "${expected_ovn_iface}" 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64
      else
        validate_system_lxd "${name}" ${num_systems} "${expected_zfs_disk}" ${expected_ceph_disks} "${expected_ovn_iface}"
      fi

      start_disk=1
      if [ -n "${expected_zfs_disk}" ]; then
        start_disk=2
      fi

      ceph_disks=""

      if [ ${expected_ceph_disks} -gt 0 ]; then
        for i in $(seq $start_disk $num_disks); do
          ceph_disks=$(echo "${ceph_disks} disk${i}" | sed -e 's/^ //')
        done
      fi

      validate_system_microceph "${name}" ${ceph_disks}
      validate_system_microovn "${name}"
    done
  }


test_interactive_combinations() {
  for num_systems in $(seq 2 4) ; do
      max_disks=2

      # A setup with 3 disks is redundant if we have less than 3 peers,
      # since we have already covered having too many disks for zfs with 2 disks per system.
      if [ ${num_systems} -ge 3 ] ; then
        max_disks=3
      fi

      for num_disks in $(seq 0 ${max_disks}) ; do
        # A setup with OVN interfaces is not necessary with fewer
        # than 3 machines as OVN setup will get skipped anyway.
        max_ifaces=0
        if [ ${num_systems} -ge 3 ]; then
          max_ifaces=2
          if [ ${num_disks} -gt 0 ]; then
            # If we are testing disks too, just stick to one interface, or none.
            max_ifaces=1
          fi
        fi

        for num_ifaces in $(seq 0 ${max_ifaces}) ; do
          # Run a test without forcibly skipping any services.
          test_case ${num_systems} ${num_disks} ${num_ifaces}

          if [ ${num_systems} -lt 3 ]; then
            if [ ${num_disks} -gt 0 ] ; then
              # If we have fewer than 3 systems, we can still create ZFS so test forcibly skipping it.
              test_case ${num_systems} ${num_disks} ${num_ifaces} "zfs"
            fi

          # Only run additional tests with skipped services if we actually have devices to set up.
          elif [ ${num_ifaces} = 1 ]; then
            if [ ${num_disks} -gt 0 ] ; then
              # Test forcibly skipping ZFS, sending available disks to Ceph instead.
              test_case ${num_systems} ${num_disks} ${num_ifaces} "zfs"
              if [ ${num_disks} -gt 1 ] ; then
                # Test forcibly skipping Ceph only if we have extra disks after ZFS setup.
                test_case ${num_systems} ${num_disks} ${num_ifaces} "ceph"
              fi

              # Test forcibly skipping both Ceph and ZFS to create no storage devices.
              test_case ${num_systems} ${num_disks} ${num_ifaces} "zfs" "ceph"

              # Test forcibly skipping Ceph, ZFS, and OVN to get a FAN device.
              test_case ${num_systems} ${num_disks} ${num_ifaces} "zfs" "ceph" "ovn"
            fi
          fi

          if [ ${num_systems} -ge 3 ] && [ ${num_ifaces} -gt 0 ]; then
              # Test forcibly skipping OVN whenever we can assign interfaces.
              test_case ${num_systems} ${num_disks} ${num_ifaces} "ovn"
          fi
        done
    done
  done
}

test_service_mismatch() {
  unset_interactive_vars
  # Selects all available systems, adds 1 local disk per system, skips ceph and ovn setup.
  LOOKUP_IFACE="enp5s0"
  LIMIT_SUBNET="yes"
  EXPECT_PEERS=2
  SETUP_ZFS="yes"
  ZFS_FILTER="lxd_disk1"
  ZFS_WIPE="yes"
  SETUP_CEPH="no"
  SETUP_OVN="no"

  # Restore the snapshots from the previous test.
  reset_systems 3 3 1

  # Install microceph and microovn on the first machine only.
  for m in micro02 micro03 ; do
    lxc exec ${m} -- sh -c "snap remove microceph --purge"
    lxc exec ${m} -- sh -c "snap remove microovn --purge"
    lxc exec ${m} -- sh -c "snap restart microcloud"
  done

  # Init should fail to find the other systems as they don't have the same services.
  # 30s should be enough time to find the other systems.
  echo "Peers with missing services won't be found after 30s"
  ! microcloud_interactive | lxc exec micro01 -- sh -c "timeout -k 5 30 microcloud init > out" || false

  # Ensure we exited while still looking for servers, and found none.
  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "Scanning for eligible servers" -q

  # Install the remaining services on the other systems.
  lxc exec micro02 -- sh -c "snap install microceph microovn"
  lxc exec micro03 -- sh -c "snap install microceph microovn"

  # Init should now work.
  echo "Creating a MicroCloud with MicroCeph and MicroOVN, but without their LXD devices"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  # Try to set up a LXD-only MicroCloud while some systems have other services present.
  reset_systems 3 3 1

  # Run all services on the other systems only.
  lxc exec micro01 -- sh -c "snap disable microceph || true"
  lxc exec micro01 -- sh -c "snap disable microovn || true"
  lxc exec micro01 -- sh -c "snap restart microcloud"


  SKIP_SERVICE="yes"
  unset SETUP_CEPH SETUP_OVN
  # Init from the minimal system should work, but not set up any services it doesn't have.
  echo "Creating a MicroCloud without setting up MicroOVN and MicroCeph on peers"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  for m in micro02 micro03 ; do
   lxc exec ${m} -- sh -c "microceph cluster list" 2>&1 | grep "Error: Daemon not yet initialized" -q
   lxc exec ${m} -- sh -c "microovn cluster list" 2>&1 | grep "Error: Daemon not yet initialized" -q
  done
}

test_disk_mismatch() {
  reset_systems 3 3 1

  # Setup micro04 with only 1 disk for ZFS.
  reset_system micro04 1 1

  echo "Creating a MicroCloud with fully remote ceph on one node"
  unset_interactive_vars
  LOOKUP_IFACE="enp5s0"
  LIMIT_SUBNET="yes"
  EXPECT_PEERS=3
  SETUP_ZFS="yes"
  ZFS_FILTER="lxd_disk1"
  ZFS_WIPE="yes"
  SETUP_CEPH="yes"
  CEPH_WARNING="yes"
  CEPH_WIPE="yes"
  SETUP_OVN="no"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep "MicroCloud is ready" -q
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

  lxc exec micro02 -- sh -c "snap stop microcloud"

  echo MicroCloud auto setup without any peers.
  ! lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out 2>&1" || false
  lxc exec micro01 -- sh -c "cat out" | tail -1 | grep -q "Error: Found no available systems"

  lxc exec micro02 -- sh -c "snap start microcloud"

  echo Auto-create a MicroCloud with 2 systems with no disks/interfaces.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Supress the first message from LXD.
    lxc exec ${m} -- sh -c "lxc ls >> /dev/null 2>&1" || true

    # Ensure we created no storage devices.
    lxc exec ${m} -- sh -ceu "lxc storage ls -f csv | wc -l | grep -q 0"
  done

  reset_systems 2 0 1

  echo Auto-create a MicroCloud with 2 systems with 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^default," || false
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^UPLINK," || false

    # Ensure we created no storage devices.
    lxc exec ${m} -- sh -ceu "lxc storage ls -f csv | wc -l | grep -q 0"
  done


  reset_systems 2 3 1

  echo Auto-create a MicroCloud with 2 systems with 3 disks and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 ; do
    validate_system_lxd "${m}" 2 disk1
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^default," || false
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^UPLINK," || false

    # Ensure we created no ceph storage devices.
    ! lxc exec ${m} -- sh -ceu "lxc storage ls -f csv" | grep -q "^remote,ceph" || false
  done

  reset_systems 3 0 0

  echo Auto-create a MicroCloud with 3 systems with no disks/interfaces.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Supress the first message from LXD.
    lxc exec ${m} -- sh -c "lxc ls >> /dev/null 2>&1" || true

    # Ensure we created no storage devices.
    lxc exec ${m} -- sh -ceu "lxc storage ls -f csv | wc -l | grep -q 0"
  done

  reset_systems 3 0 1

  echo Auto-create a MicroCloud with 3 systems with 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03; do
    validate_system_lxd "${m}" 3
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^default," || false
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^UPLINK," || false

    # Ensure we created no storage devices.
    lxc exec ${m} -- sh -ceu "lxc storage ls -f csv | wc -l | grep -q 0"
  done

  reset_systems 3 1 1

  echo Auto-create a MicroCloud with 3 systems with 1 disk and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03; do
    validate_system_lxd "${m}" 3 "" 1
    validate_system_microceph "${m}" disk1
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^default," || false
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^UPLINK," || false

    # Ensure we created no zfs storage devices.
    ! lxc exec ${m} -- sh -ceu "lxc storage ls -f csv" | grep -q "^local,zfs" || false
  done

  reset_systems 3 3 1

  echo Auto-create a MicroCloud with 3 systems with 3 disks and 1 interface each.
  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 2
    validate_system_microceph "${m}" disk2 disk3
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^default," || false
    ! lxc exec ${m} -- sh -c "lxc network ls -f csv" | grep -q "^UPLINK," || false
  done
}
