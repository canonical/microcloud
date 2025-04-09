#!/bin/bash

check_instance_connectivity() {
  instance_1="${1:-}"
  instance_2="${2:-}"
  is_vm="${3:-0}"

  skip_instance() {
    [ "${is_vm}" = "1" ] && [ "${SKIP_VM_LAUNCH}" = "1" ]
  }

  if skip_instance ; then
    echo "Skipping VM ${instance_1} because SKIP_VM_LAUNCH=1 is set"
    return 0
  fi

  # Ensure we can reach the launched instances.
  for m in "${instance_1}" "${instance_2}" ; do
    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 60); do
      if [ \$(lxc list -f csv -c s ${m}) = 'READY' ]; then
         lxc exec ${m} -- test -d /cephfs
         echo \" ${m} booted successfully\"

         return 0
      fi
      echo -n .
      sleep 5
    done
    echo FAIL
    return 1
    "
  done

  echo "Test connectivity to lxdbr0 via DNS, IPv4 and IPv6"
  IPV4_GW="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
  IPV6_GW="$(lxc network get lxdbr0 ipv6.address | cut -d/ -f1)"
  for m in "${instance_1}" "${instance_2}" ; do
    lxc exec micro01 -- lxc exec "${m}" -- timeout 5 bash -cex "for dst in _gateway ${IPV4_GW} ${IPV6_GW}; do grep -qm1 ^SSH- < /dev/tcp/\$dst/22; done"
  done

  echo "Test connectivity between instances"
  lxc exec micro01 -- lxc exec "${instance_1}" -- ping -nc1 -w5 -4 "${instance_2}"
  lxc exec micro01 -- lxc exec "${instance_1}" -- ping -nc1 -w5 -6 "${instance_2}"
  lxc exec micro01 -- lxc exec "${instance_2}" -- ping -nc1 -w5 -4 "${instance_1}"
  lxc exec micro01 -- lxc exec "${instance_2}" -- ping -nc1 -w5 -6 "${instance_1}"
}


test_instances_config() {
  reset_systems 3 3 2

  # Setup a MicroCloud with 3 systems, ZFS storage, and a FAN network.
  addr=$(lxc ls micro01 -f csv -c4 | awk '/enp5s0/ {print $1}')
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
- name: micro02
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
- name: micro03
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
- name: micro03
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
ceph:
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  # Delete any instances left behind.
  lxc exec micro01 -- sh -c "
  for m in \$(lxc ls -f csv -c n) ; do
    lxc rm \$m -f
  done
"

  # Launch a container and VM with CEPH storage & OVN network.
  lxc exec micro01 -- lxc init ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  lxc exec micro01 -- lxc init ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=3GiB -s remote -n default

  # Ensure proper storage pool and network selection by inspecting their used_by.
  for m in c1 v1 ; do
      lxc exec micro01 -- lxc storage show remote  | grep -xF -- "- /1.0/instances/${m}"
      lxc exec micro01 -- lxc network show default | grep -xF -- "- /1.0/instances/${m}"
  done
}

test_instances_launch() {
  reset_systems 3 3 2

  # Setup a MicroCloud with 3 systems, ZFS storage, and a FAN network.
  addr=$(lxc ls micro01 -f csv -c4 | awk '/enp5s0/ {print $1}')
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
- name: micro02
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
- name: micro03
  ovn_uplink_interface: enp6s0
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
EOF

  # Launch a container and VM with ZFS storage & FAN network.
  if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    echo "::warning::SKIPPING VM LAUNCH TEST"
  else
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s local -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=2GiB -s local -n default

  # Ensure we can reach the launched instances.
  for m in c1 v1 ; do
    if [ "${m}" = "v1" ] && [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      continue
    fi

    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 60); do
      if [ \$(lxc list -f csv -c s ${m}) = 'READY' ]; then
         echo \" ${m} booted successfully\"

         lxc rm ${m} -f
         return 0
      fi
      echo -n .
      sleep 5
    done
    echo FAIL
    return 1
    "
  done

  reset_systems 3 3 2

  # Create a MicroCloud with ceph and ovn setup.
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
- name: micro03
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
ceph:
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c2 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default

  check_instance_connectivity "c1" "c2" "0"
  check_instance_connectivity "v1" "c1" "1"

  lxc exec micro01 -- lxc delete -f c1 c2
  if ! [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    lxc exec micro01 -- lxc delete -f v1
  fi

  # Create a MicroCloud with ceph, partially disaggregated ceph networking and ovn setup.
  reset_systems 3 3 3
  addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

  ceph_dedicated_subnet_prefix="10.0.1"
  ceph_dedicated_subnet_iface="enp7s0"

  for n in $(seq 2 4); do
    dedicated_ip="${ceph_dedicated_subnet_prefix}.${n}/24"
    lxc exec "micro0$((n-1))" -- ip addr add "${dedicated_ip}" dev "${ceph_dedicated_subnet_iface}"
  done

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
- name: micro03
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
ceph:
  internal_network: ${ceph_dedicated_subnet_prefix}.0/24
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c2 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default

  check_instance_connectivity "c1" "c2" "0"
  check_instance_connectivity "v1" "c1" "1"

  lxc exec micro01 -- lxc delete -f c1 c2
  if ! [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    lxc exec micro01 -- lxc delete -f v1
  fi

  # Create a MicroCloud with ceph, fully disaggregated ceph networking and ovn underlay network.
  reset_systems 3 3 5
  addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.0.2"
  ceph_public_subnet_iface="enp8s0"
  ovn_underlay_subnet_prefix="10.0.3"
  ovn_underlay_subnet_iface="enp9s0"
  set_cluster_subnet 3  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 3  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
  set_cluster_subnet 3  "${ovn_underlay_subnet_iface}" "${ovn_underlay_subnet_prefix}"

  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_underlay_ip: 10.0.3.2
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
- name: micro02
  ovn_underlay_ip: 10.0.3.3
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
- name: micro03
  ovn_underlay_ip: 10.0.3.4
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
ceph:
  internal_network: ${ceph_cluster_subnet_prefix}.0/24
  public_network: ${ceph_public_subnet_prefix}.0/24
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c2 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default

  check_instance_connectivity "c1" "c2" "0"
  check_instance_connectivity "v1" "c1" "1"

  lxc exec micro01 -- lxc delete -f c1 c2
  if ! [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    lxc exec micro01 -- lxc delete -f v1
  fi

  # Create a MicroCloud with ceph, fully disaggregated ceph networking and ovn underlay network with an extended cluster.
  reset_systems 4 3 5
  addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

  ceph_cluster_subnet_prefix="10.0.1"
  ceph_cluster_subnet_iface="enp7s0"
  ceph_public_subnet_prefix="10.0.2"
  ceph_public_subnet_iface="enp8s0"
  ovn_underlay_subnet_prefix="10.0.3"
  ovn_underlay_subnet_iface="enp9s0"
  set_cluster_subnet 4 "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
  set_cluster_subnet 4 "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
  set_cluster_subnet 4 "${ovn_underlay_subnet_iface}" "${ovn_underlay_subnet_prefix}"

  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_underlay_ip: 10.0.3.2
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
- name: micro02
  ovn_underlay_ip: 10.0.3.3
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
ceph:
  internal_network: ${ceph_cluster_subnet_prefix}.0/24
  public_network: ${ceph_public_subnet_prefix}.0/24
  cephfs: true
EOF
  )"

  lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro03
  ovn_underlay_ip: 10.0.3.4
  ovn_uplink_interface: enp6s0
  storage:
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
EOF
  )"

  lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

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
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 v1 -c limits.memory=512MiB -d root,size=3GiB --vm -s remote -n default --target micro03
  fi
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c1 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default --target micro01
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c2 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default --target micro03

  check_instance_connectivity "c1" "c2" "0"
  check_instance_connectivity "v1" "c1" "1"

  # We're done with c2 so we can delete it now, and free up space on the runners for more instances.
  lxc exec micro01 -- lxc delete c2 -f

  # Flush the neighbour cache in the remaining container so that it can ping new containers via hostname quicker.
  lxc exec micro01 -- lxc exec c1 -- ip neigh flush all


  # Add a new node, don't add any OSDs to keep Ceph fully remote.
  preseed="$(cat << EOF
lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_underlay_ip: 10.0.3.5
  ovn_uplink_interface: enp6s0
EOF
  )"

  lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
  lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c3 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default --target micro01
  lxc exec micro01 -- lxc launch ubuntu-minimal-daily:22.04 c4 -c limits.memory=512MiB -d root,size=2GiB -s remote -n default --target micro04

  # Let cloud-init finish its job of installing required packages (i.e: iputils-ping).
  for m in c3 c4; do
    echo -n "Waiting up to 5 mins for ${m} to start "
    lxc exec micro01 -- sh -ceu "
    for round in \$(seq 60); do
      if [ \$(lxc list -f csv -c s ${m}) = 'READY' ]; then
         echo \" ${m} booted successfully\"

         return 0
      fi
      echo -n .
      sleep 5
    done
    echo FAIL
    return 1
    "
  done

  lxc exec micro01 -- lxc stop c3
  lxc exec micro01 -- lxc move c3 --target micro04
  lxc exec micro01 -- lxc start c3

  check_instance_connectivity "c1" "c3" "0"
  check_instance_connectivity "c1" "c4" "0"

  lxc exec micro01 -- lxc delete -f c1 c3 c4
  if ! [ "${SKIP_VM_LAUNCH}" = "1" ]; then
    lxc exec micro01 -- lxc delete -f v1
  fi
}
