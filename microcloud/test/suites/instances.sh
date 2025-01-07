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
    for round in \$(seq 100); do
      if lxc list -f csv -c s ${m} | grep -qxF READY; then
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

  for m in "${instance_1}" "${instance_2}" ; do
    lxc exec micro01 -- lxc exec "${m}" -- apt-get update
    lxc exec micro01 -- lxc exec "${m}" -- apt-get install -y --no-install-recommends iputils-ping

    echo "Test connectivity to lxdbr0"
    IPV4_GW="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
    IPV6_GW="$(lxc network get lxdbr0 ipv6.address | cut -d/ -f1)"

    lxc exec micro01 -- lxc exec "${m}" -- ping -nc1 -w5 -4 "${IPV4_GW}"
    lxc exec micro01 -- lxc exec "${m}" -- ping -nc1 -w5 -6 "${IPV6_GW}"
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
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  preseed="
lookup_subnet: ${addr}/24
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
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"

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
  preseed="
lookup_subnet: ${addr}/24
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
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"

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
  addr=$(lxc ls micro01 -f csv -c4 | grep enp5s0 | cut -d' ' -f1)
  preseed="
lookup_subnet: ${addr}/24
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
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"

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
  preseed="
lookup_subnet: ${addr}/24
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
"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud init --preseed <<< "${preseed}"

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
}
