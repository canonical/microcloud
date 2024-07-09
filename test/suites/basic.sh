#!/bin/bash

test_interactive() {
  reset_systems 3 3 1

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with all services but no devices"
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="no"
  export SETUP_CEPH="no"
  export SETUP_OVN="no"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
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

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with ZFS storage"
  export SKIP_SERVICE="yes"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  unset SETUP_CEPH SETUP_OVN
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems with just LXD and no IPv6 support.
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- echo 1 > /proc/sys/net/ipv6/conf/all/disable_ipv6
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

  echo "Creating a MicroCloud with ZFS storage and no IPv6 support"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1
  done

  # Reset the systems with just LXD and no IPv4 support.
  gw_net_addr=$(lxc network get lxdbr0 ipv4.address)
  lxc network set lxdbr0 ipv4.address none
  reset_systems 3 3 1

  for m in micro01 micro02 micro03 ; do
    lxc exec "${m}" -- snap disable microceph || true
    lxc exec "${m}" -- snap disable microovn || true
    lxc exec "${m}" -- snap restart microcloud
  done

  export PROCEED_WITH_NO_OVERLAY_NETWORKING="no" # This will avoid to setup the cluster if no overlay networking is available.
  echo "Creating a MicroCloud with ZFS storage and no IPv4 support"
  ! microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init 2> err" || false

  # Ensure we error out due to a lack of usable overlay networking.
  lxc exec micro01 -- cat err | grep "cluster bootstrapping aborted due to lack of usable networking" -q

  # Set the IPv4 address back to the original value.
  lxc network set lxdbr0 ipv4.address "${gw_net_addr}"
  unset PROCEED_WITH_NO_OVERLAY_NETWORKING

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
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

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

  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

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
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
    validate_system_microceph "${m}" 1 disk2
    validate_system_microovn "${m}"
  done

  # Reset the systems and install microovn and microceph with a partially disaggregated ceph network setup.
  reset_systems 3 3 3

  ceph_dedicated_subnet_prefix="10.0.1"
  ceph_dedicated_subnet_iface="enp7s0"

  for n in $(seq 2 4); do
    dedicated_ip="${ceph_dedicated_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${dedicated_ip}" dev "${ceph_dedicated_subnet_iface}"
  done

  echo "Creating a MicroCloud with ZFS, Ceph storage with a partially disaggregated Ceph networking setup, and OVN network"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${ceph_dedicated_subnet_prefix}.0/24"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" 1 "${ceph_dedicated_subnet_prefix}.0/24" disk2
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

  echo "Creating a MicroCloud with ZFS, Ceph storage with a fully disaggregated Ceph networking setup, and OVN network"
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export CEPH_CLUSTER_NETWORK="${ceph_cluster_subnet_prefix}.0/24"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}"
    validate_system_microceph "${m}" 1 "${CEPH_CLUSTER_NETWORK}" disk2
    validate_system_microovn "${m}"
  done
}

test_instances_config() {
  reset_systems 3 3 2

  # Setup a MicroCloud with 3 systems, ZFS storage, and a FAN network.
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed << EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
systems:
- name: micro01
  storage:
    local:
      path: /dev/sdb
      wipe: true
- name: micro02
  storage:
    local:
      path: /dev/sdb
      wipe: true
- name: micro03
  storage:
    local:
      path: /dev/sdb
      wipe: true
EOF

  # Init a container and VM with ZFS storage & FAN network.
  lxc exec micro01 -- lxc init --empty v1 --vm
  lxc exec micro01 -- lxc init --empty c1

  # Ensure proper storage pool and network selection by inspecting their used_by.
  for m in c1 v1 ; do
      lxc exec micro01 -- lxc storage show local   | grep -xF -- "- /1.0/instances/${m}"
      lxc exec micro01 -- lxc network show lxdfan0 | grep -xF -- "- /1.0/instances/${m}"
  done

  reset_systems 3 3 2

  # Create a MicroCloud with ceph and ovn setup.
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed << EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
systems:
- name: micro01
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro02
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro03
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
storage:
  cephfs: true
EOF

  # Delete any instances left behind.
  lxc exec micro01 -- sh -c "
  for m in \$(lxc ls -f csv -c n) ; do
    lxc rm \$m -f
  done
"

  # Launch a container and VM with CEPH storage & OVN network.
  lxc exec micro01 -- lxc init ubuntu-minimal:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  lxc exec micro01 -- lxc init ubuntu-minimal:22.04 c1 -c limits.memory=512MiB -d root,size=3GiB -s remote -n default

  # Ensure proper storage pool and network selection by inspecting their used_by.
  for m in c1 v1 ; do
      lxc exec micro01 -- lxc storage show remote  | grep -xF -- "- /1.0/instances/${m}"
      lxc exec micro01 -- lxc network show default | grep -xF -- "- /1.0/instances/${m}"
  done
}

test_instances_launch() {
  reset_systems 3 3 2

  # Setup a MicroCloud with 3 systems, ZFS storage, and a FAN network.
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed << EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
systems:
- name: micro01
  storage:
    local:
      path: /dev/sdb
      wipe: true
- name: micro02
  storage:
    local:
      path: /dev/sdb
      wipe: true
- name: micro03
  storage:
    local:
      path: /dev/sdb
      wipe: true
EOF

  # Delete any instances left behind.
  lxc exec micro01 -- sh -c "
  for m in \$(lxc ls -f csv -c n) ; do
    lxc rm \$m -f
  done
"

  # Add cloud-init entry for checking ready state on launched instances.
  lxc exec micro01 -- lxc profile edit default << EOF
config:
  cloud-init.user-data: |
    #cloud-config
    write_files:
      - content: |
          #!/bin/sh
          exec curl --unix-socket /dev/lxd/sock lxd/1.0 -X PATCH -d '{"state": "Ready"}'
        path: /var/lib/cloud/scripts/per-boot/ready.sh
        permissions: "0755"
EOF

  # Launch a container and VM with ZFS storage & FAN network.
  if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    echo "::warning::SKIPPING VM LAUNCH TEST"
  else
    lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s local -n lxdfan0
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 c1 -c limits.memory=512MiB -d root,size=1GiB -s local -n lxdfan0

  # Ensure we can reach the launched instances.
  for m in c1 v1 ; do
    if [ "${m}" = "v1" ] && [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      continue
    fi

    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 100); do
      if lxc list -f csv -c s ${m} | grep -qxF READY; then
         echo \" ${m} booted successfully\"

         lxc rm ${m} -f
         return 0
      fi
      echo -n .
      sleep 3
    done
    echo FAIL
    return 1
    "
  done

  reset_systems 3 3 2

  # Create a MicroCloud with ceph and ovn setup.
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed << EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
systems:
- name: micro01
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro02
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro03
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
storage:
  cephfs: true
EOF

  # Delete any instances left behind.
  lxc exec micro01 -- sh -c "
  for m in \$(lxc ls -f csv -c n) ; do
    lxc rm \$m -f
  done
"
  # Add cloud-init entry for checking ready state on launched instances.
  lxc exec micro01 -- lxc profile edit default << EOF
config:
  cloud-init.user-data: |
    #cloud-config
    packages:
    - iputils-ping
    write_files:
      - content: |
          #!/bin/sh
          exec curl --unix-socket /dev/lxd/sock lxd/1.0 -X PATCH -d '{"state": "Ready"}'
        path: /var/lib/cloud/scripts/per-boot/ready.sh
        permissions: "0755"
devices:
  fs:
    ceph.cluster_name: ceph
    ceph.user_name: admin
    path: /cephfs
    source: cephfs:lxd_cephfs/
    type: disk
EOF

  # Launch 2 containers and VM with CEPH storage & OVN network.
  if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    echo "::warning::SKIPPING VM LAUNCH TEST"
  else
    lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 c1 -c limits.memory=512MiB -d root,size=1GiB -s remote -n default
  lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 c2 -c limits.memory=512MiB -d root,size=1GiB -s remote -n default

  # Ensure we can reach the launched instances.
  for m in c1 c2 v1 ; do
    if [ "${m}" = "v1" ] && [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      continue
    fi

    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 100); do
      if lxc list -f csv -c s ${m} | grep -qxF READY; then
         lxc exec ${m} -- stat /cephfs
         echo \" ${m} booted successfully\"

         return 0
      fi
      echo -n .
      sleep 3
    done
    echo FAIL
    return 1
    "
  done

  echo "Test connectivity to lxdbr0"
  IPV4_GW="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
  IPV6_GW="$(lxc network get lxdbr0 ipv6.address | cut -d/ -f1)"

  lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -4 "${IPV4_GW}"
  lxc exec micro01 -- lxc exec c2 -- ping -nc1 -w5 -4 "${IPV4_GW}"
  lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -6 "${IPV6_GW}"
  lxc exec micro01 -- lxc exec c2 -- ping -nc1 -w5 -6 "${IPV6_GW}"
  if [ "${SKIP_VM_LAUNCH}" != "1" ]; then
    lxc exec micro01 -- lxc exec v1 -- ping -nc1 -w5 -4 "${IPV4_GW}"
    lxc exec micro01 -- lxc exec v1 -- ping -nc1 -w5 -6 "${IPV6_GW}"
  fi

  echo "Test connectivity between instances"
  lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -4 c2
  lxc exec micro01 -- lxc exec c2 -- ping -nc1 -w5 -4 c1
  lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -6 c2
  lxc exec micro01 -- lxc exec c2 -- ping -nc1 -w5 -6 c1
  if [ "${SKIP_VM_LAUNCH}" != "1" ]; then
    lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -4 v1
    lxc exec micro01 -- lxc exec v1 -- ping -nc1 -w5 -4 c1
    lxc exec micro01 -- lxc exec c1 -- ping -nc1 -w5 -6 v1
    lxc exec micro01 -- lxc exec v1 -- ping -nc1 -w5 -6 c1

    lxc exec micro01 -- lxc delete -f v1
  fi

  lxc exec micro01 -- lxc delete -f c1 c2

  # Create a MicroCloud with ceph, partially disaggregated ceph networking and ovn setup.
  reset_systems 3 3 3
  addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

  ceph_dedicated_subnet_prefix="10.0.1"
  ceph_dedicated_subnet_iface="enp7s0"

  for n in $(seq 2 4); do
    dedicated_ip="${ceph_dedicated_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${dedicated_ip}" dev "${ceph_dedicated_subnet_iface}"
  done

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
systems:
- name: micro01
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro02
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
- name: micro03
  storage:
    ceph:
      - path: /dev/sdc
        wipe: true
      - path: /dev/sdd
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
ceph:
  internal_network: ${ceph_dedicated_subnet_prefix}.0/24
storage:
  cephfs: true
EOF

  # Add cloud-init entry for checking ready state on launched instances.
  lxc exec micro01 -- lxc profile edit default << EOF
config:
  cloud-init.user-data: |
    #cloud-config
    packages:
    - iputils-ping
    write_files:
      - content: |
          #!/bin/sh
          exec curl --unix-socket /dev/lxd/sock lxd/1.0 -X PATCH -d '{"state": "Ready"}'
        path: /var/lib/cloud/scripts/per-boot/ready.sh
        permissions: "0755"
devices:
  fs:
    ceph.cluster_name: ceph
    ceph.user_name: admin
    path: /cephfs
    source: cephfs:lxd_cephfs/
    type: disk
EOF

  # Launch a container and VM with CEPH storage & OVN network.
  if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    echo "::warning::SKIPPING VM LAUNCH TEST"
  else
    lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal:22.04 c1 -c limits.memory=512MiB -d root,size=1GiB -s remote -n default

  # Ensure we can reach the launched instances.
  for m in c1 v1 ; do
    if [ "${m}" = "v1" ] && [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      continue
    fi

    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 100); do
      if lxc info ${m} | grep -qxF 'Status: READY'; then
         lxc exec ${m} -- stat /cephfs
         echo \" ${m} booted successfully\"

         return 0
      fi
      echo -n .
      sleep 3
    done
    echo FAIL
    return 1
    "
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

    LOOKUP_IFACE="enp5s0" # filter string for the lookup interface table.
    LIMIT_SUBNET="yes" # (yes/no) input for limiting lookup of systems to the above subnet.
    CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"

    EXPECT_PEERS="$((num_systems - 1))"

    if [ "${num_disks}" -gt 0 ] ; then
      if [ -z "${force_no_zfs}" ]; then
        SETUP_ZFS="yes"
        ZFS_FILTER="disk1"
        ZFS_WIPE="yes"
        expected_zfs_disk="disk1"
      else
        SETUP_ZFS="no"
      fi
    fi

    if [ "${num_disks}" -gt 0 ] && [ "${num_systems}" -ge 3 ] ; then
      # If we only have one disk and we used it for ZFS, there should be no prompt.
      if [ "${num_disks}" = 1 ] && [ -z "${force_no_zfs}" ] ; then
        echo "Insufficient disks"
      elif [ -z "${force_no_ceph}" ]; then
        SETUP_CEPH="yes"
        SETUP_CEPHFS="yes"
        CEPH_WIPE="yes"
        expected_ceph_disks="${num_disks}"
        if [ -n "${expected_zfs_disk}" ]; then
          expected_ceph_disks="$((num_disks - 1))"
        fi

        if [ "${expected_ceph_disks}" -gt 0 ]; then
          expected_cephfs=1
        fi
      else
        SETUP_CEPH="no"
      fi
    fi

    if [ "${num_ifaces}" -gt 0 ] && [ "${num_systems}" -ge 3 ] ; then
      if [ -z "${force_no_ovn}" ] ; then
        SETUP_OVN="yes"

        # Always pick the first available interface.
        OVN_FILTER="enp6s0"
        IPV4_SUBNET="10.1.123.1/24"
        IPV4_START="10.1.123.100"
        IPV4_END="10.1.123.254"
        IPV6_SUBNET="fd42:1:1234:1234::1/64"
        DNS_ADDRESSES="10.1.123.1,8.8.8.8"

        expected_ovn_iface="enp6s0"
      else
        SETUP_OVN="no"
      fi
    fi

    microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
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
  for num_systems in $(seq 2 4) ; do
      max_disks=2

      # A setup with 3 disks is redundant if we have less than 3 peers,
      # since we have already covered having too many disks for zfs with 2 disks per system.
      if [ "${num_systems}" -ge 3 ] ; then
        max_disks=3
      fi

      for num_disks in $(seq 0 "${max_disks}") ; do
        # A setup with OVN interfaces is not necessary with fewer
        # than 3 machines as OVN setup will get skipped anyway.
        max_ifaces=0
        if [ "${num_systems}" -ge 3 ]; then
          max_ifaces=2
          if [ "${num_disks}" -gt 0 ]; then
            # If we are testing disks too, just stick to one interface, or none.
            max_ifaces=1
          fi
        fi

        for num_ifaces in $(seq 0 "${max_ifaces}") ; do
          # Run a test without forcibly skipping any services.
          _test_case "${num_systems}" "${num_disks}" "${num_ifaces}"

          if [ "${num_systems}" -lt 3 ]; then
            if [ "${num_disks}" -gt 0 ] ; then
              # If we have fewer than 3 systems, we can still create ZFS so test forcibly skipping it.
              _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "zfs"
            fi

          # Only run additional tests with skipped services if we actually have devices to set up.
          elif [ "${num_ifaces}" = 1 ]; then
            if [ "${num_disks}" -gt 0 ] ; then
              # Test forcibly skipping ZFS, sending available disks to Ceph instead.
              _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "zfs"
              if [ "${num_disks}" -gt 1 ] ; then
                # Test forcibly skipping Ceph only if we have extra disks after ZFS setup.
                _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "ceph"
              fi

              # Test forcibly skipping both Ceph and ZFS to create no storage devices.
              _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "zfs" "ceph"

              # Test forcibly skipping Ceph, ZFS, and OVN to get a FAN device.
              _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "zfs" "ceph" "ovn"
            fi
          fi

          if [ "${num_systems}" -ge 3 ] && [ "${num_ifaces}" -gt 0 ]; then
              # Test forcibly skipping OVN whenever we can assign interfaces.
              _test_case "${num_systems}" "${num_disks}" "${num_ifaces}" "ovn"
          fi
        done
    done
  done
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

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  export CEPH_PUBLIC_NETWORK="${microcloud_internal_net_addr}"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"

  # Install microceph and microovn on the first machine only.
  for m in micro02 micro03 ; do
    lxc exec "${m}" -- snap remove microceph --purge
    lxc exec "${m}" -- snap remove microovn --purge
    lxc exec "${m}" -- snap restart microcloud
  done

  # Init should fail to find the other systems as they don't have the same services.
  # 30s should be enough time to find the other systems.
  echo "Peers with missing services won't be found after 30s"
  ! microcloud_interactive | lxc exec micro01 -- sh -c "timeout -k 5 30 microcloud init > out" || false

  # Ensure we exited while still looking for servers, and found none.
  lxc exec micro01 -- tail -1 out | grep "Scanning for eligible servers" -q

  # Install the remaining services on the other systems.
  lxc exec micro02 -- snap install microceph --channel="${MICROCEPH_SNAP_CHANNEL}" --cohort="+"
  lxc exec micro02 -- snap install microovn  --channel="${MICROOVN_SNAP_CHANNEL}"  --cohort="+"
  lxc exec micro03 -- snap install microceph --channel="${MICROCEPH_SNAP_CHANNEL}" --cohort="+"
  lxc exec micro03 -- snap install microovn  --channel="${MICROOVN_SNAP_CHANNEL}"  --cohort="+"

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
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=3
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_WARNING="yes"
  export CEPH_WIPE="yes"
  export SETUP_OVN="no"
  export CEPH_CLUSTER_NETWORK="${microcloud_internal_net_addr}"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
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
    validate_system_lxd "${m}" 3 "" 1 0
    validate_system_microceph "${m}" 0 disk1
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
    validate_system_lxd "${m}" 3 disk1 2 0
    validate_system_microceph "${m}" 0 disk2 disk3
    validate_system_microovn "${m}"

    # Ensure we didn't create any other network devices.
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^default," || false
    ! lxc exec ${m} -- lxc network ls -f csv | grep -q "^UPLINK," || false
  done
}

# services_validator: A basic validator of 3 systems with typical expected inputs.
services_validator() {
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd ${m} 3 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8
    validate_system_microceph ${m} 1 disk2
    validate_system_microovn ${m}
  done
}

test_reuse_cluster() {
  unset_interactive_vars

  # Set the default config for interactive setup.
  export LOOKUP_IFACE="enp5s0"
  export LIMIT_SUBNET="yes"
  export EXPECT_PEERS=2
  export SETUP_ZFS="yes"
  export ZFS_FILTER="lxd_disk1"
  export ZFS_WIPE="yes"
  export SETUP_CEPH="yes"
  export SETUP_CEPHFS="yes"
  export CEPH_FILTER="lxd_disk2"
  export CEPH_WIPE="yes"
  export SETUP_OVN="yes"
  export OVN_FILTER="enp6s0"
  export IPV4_SUBNET="10.1.123.1/24"
  export IPV4_START="10.1.123.100"
  export IPV4_END="10.1.123.254"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service"
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="add"
  lxc exec micro02 -- microceph cluster bootstrap
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service on the local node"
  lxc exec micro01 -- microceph cluster bootstrap
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing MicroCeph and MicroOVN"
  export REUSE_EXISTING_COUNT=2
  export REUSE_EXISTING="add"
  lxc exec micro02 -- microceph cluster bootstrap
  lxc exec micro02 -- microovn cluster bootstrap
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing MicroCeph and MicroOVN on different nodes"
  lxc exec micro02 -- microceph cluster bootstrap
  lxc exec micro03 -- microovn cluster bootstrap
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service with multiple nodes from this cluster"
  export REUSE_EXISTING_COUNT=1
  export REUSE_EXISTING="add"
  lxc exec micro02 -- microceph cluster bootstrap
  token="$(lxc exec micro02 -- microceph cluster add micro01)"
  lxc exec micro01 -- microceph cluster join "${token}"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing existing service with all nodes from this cluster"
  lxc exec micro02 -- microceph cluster bootstrap
  token="$(lxc exec micro02 -- microceph cluster add micro01)"
  lxc exec micro01 -- microceph cluster join "${token}"
  token="$(lxc exec micro02 -- microceph cluster add micro03)"
  lxc exec micro03 -- microceph cluster join "${token}"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator

  reset_systems 4 3 3
  echo "Create a MicroCloud that re-uses an existing existing service with foreign cluster members"
  lxc exec micro04 -- snap disable microcloud
  lxc exec micro02 -- microceph cluster bootstrap
  token="$(lxc exec micro02 -- microceph cluster add micro04)"
  lxc exec micro04 -- microceph cluster join "${token}"
  microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out"
  services_validator
  validate_system_microceph micro04 1

  reset_systems 3 3 3
  echo "Fail to create a MicroCloud due to an existing service if --auto specified"
  lxc exec micro02 -- microceph cluster bootstrap
  ! lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out" || true



  echo "Fail to create a MicroCloud due to conflicting existing services"
  lxc exec micro03 -- microceph cluster bootstrap
  ! microcloud_interactive | lxc exec micro01 -- sh -c "microcloud init > out" || true

  reset_systems 3 3 3
  echo "Create a MicroCloud that re-uses an existing service with preseed"
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  lxc exec micro01 --env TEST_CONSOLE=0 --  microcloud init --preseed << EOF
lookup_subnet: ${addr}/24
lookup_interface: enp5s0
reuse_existing_clusters: true
systems:
- name: micro01
- name: micro02
- name: micro03
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8
storage:
  local:
    - find: id == sdb
      wipe: true
  ceph:
    - find: id == sdc
      wipe: true
  cephfs: true
EOF

  services_validator
}
