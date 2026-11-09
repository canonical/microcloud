#!/bin/bash

# debug_tcp: Print OK/FAILED for a TCP connect from the given exec prefix to host:port.
# Usage: debug_tcp <label> <host> <port> <cmd prefix...>
debug_tcp() {
  local label="${1}" dst="${2}" port="${3}"
  shift 3
  "$@" timeout 5 bash -c "exec 3<>/dev/tcp/${dst}/${port}" && echo "${label} tcp ${dst}:${port} OK" || echo "${label} tcp ${dst}:${port} FAILED"
}

# debug_host_ssh: Print state of SSH on the runner and connectivity from the runner, micro01 and c1 to the host's lxdbr0 address.
debug_host_ssh() {
  local label="${1}"
  local gw c1gw uplinkgw ovnrouters r
  gw="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
  echo "::group::debug-host-ssh (${label})"
  set +e

  # Install tcpdump on the runner and micro01 if missing.
  command -v tcpdump > /dev/null || { sudo apt-get update -qq; sudo apt-get install -y -qq tcpdump; }
  lxc exec micro01 -- bash -c "command -v tcpdump > /dev/null || { apt-get update -qq; apt-get install -y -qq tcpdump; }"

  systemctl is-active ssh.service ssh.socket sshd.service
  systemctl is-enabled ssh.service ssh.socket
  ss -tlnp | grep -E ':(22|8443)\b'
  dpkg -l openssh-server | tail -1
  ip -br addr show lxdbr0

  echo "--- c1 network state"
  lxc exec micro01 -- lxc exec c1 -- ip -br addr
  lxc exec micro01 -- lxc exec c1 -- ip route
  lxc exec micro01 -- lxc exec c1 -- ip -6 route
  lxc exec micro01 -- lxc exec c1 -- ip neigh
  c1gw="$(lxc exec micro01 -- lxc exec c1 -- ip -4 route show default | awk '/default/ {print $3; exit}')"
  uplinkgw="$(lxc exec micro01 -- lxc network get UPLINK ipv4.gateway | cut -d/ -f1)"
  echo "c1 default gateway: ${c1gw}, uplink gateway: ${uplinkgw}, host lxdbr0: ${gw}"

  echo "--- LXD networks (micro01)"
  lxc exec micro01 -- lxc network list
  lxc exec micro01 -- lxc network list --format csv -c n | while read -r n; do lxc exec micro01 -- lxc network show "${n}"; done
  lxc exec micro01 -- lxc config show c1 --expanded

  echo "--- OVN (micro01)"
  lxc exec micro01 -- microovn.ovn-nbctl show
  ovnrouters="$(lxc exec micro01 -- microovn.ovn-nbctl lr-list | awk '{gsub(/[()]/,"",$2); print $2}')"
  for r in ${ovnrouters}; do
    echo "router ${r}"
    lxc exec micro01 -- microovn.ovn-nbctl lr-route-list "${r}"
    lxc exec micro01 -- microovn.ovn-nbctl lr-nat-list "${r}"
    lxc exec micro01 -- microovn.ovn-nbctl lr-policy-list "${r}"
  done
  lxc exec micro01 -- microovn.ovn-nbctl list Gateway_Chassis
  lxc exec micro01 -- microovn.ovn-sbctl show
  lxc exec micro01 -- microovn.ovn-sbctl find Port_Binding type=chassisredirect
  lxc exec micro01 -- microovn.ovn-nbctl list HA_Chassis_Group
  lxc exec micro01 -- microovn.ovn-nbctl list HA_Chassis
  lxc exec micro01 -- microovn.ovn-sbctl list Chassis
  lxc exec micro01 -- microovn.ovn-nbctl --columns=_uuid,name,options,gateway_chassis,ha_chassis_group list Logical_Router_Port lxd-net2-lr-lrp-ext
  lxc exec micro01 -- microovn.ovn-appctl -t ovn-controller ct-zone-list
  echo "--- chassisredirect binding poll (30s)"
  for _ in $(seq 1 30); do
    echo "$(date +%T) chassis=$(lxc exec micro01 -- microovn.ovn-sbctl --bare --columns=chassis find Port_Binding type=chassisredirect | tr '\n' ' ')"
    sleep 1
  done
  lxc exec micro01 -- bash -c "journalctl --no-pager -n 300 -u snap.microovn.chassis.service 2>/dev/null | grep -iE 'claim|release|cr-lxd|ha_chassis'"
  lxc exec micro01 -- microovn.ovs-vsctl get open_vswitch . external_ids
  lxc exec micro01 -- ip -br addr
  lxc exec micro01 -- ip route

  echo "--- host routing and filtering"
  ip route
  ip route get 10.1.123.100
  ip route get "${gw}"
  sysctl net.ipv4.ip_forward net.ipv4.conf.all.rp_filter net.ipv4.conf.default.rp_filter net.ipv4.conf.microbr0.rp_filter net.ipv4.conf.lxdbr0.rp_filter net.ipv4.conf.microbr0.forwarding net.ipv4.conf.lxdbr0.forwarding
  nft list tables
  nft list ruleset | grep -B2 -A10 microbr0
  sudo iptables-save
  conntrack -L 2> /dev/null | grep -E "10\.1\.123\.100|10\.95\." | head -20

  echo "--- hop-by-hop tests"
  debug_tcp "host->lxdbr0" "${gw}" 22
  debug_tcp "micro01->lxdbr0" "${gw}" 22 lxc exec micro01 --
  debug_tcp "c1->uplinkgw" "${uplinkgw}" 22 lxc exec micro01 -- lxc exec c1 --

  echo "--- c1->lxdbr0:22 with packet captures"
  sudo timeout 8 tcpdump -ni any -c 60 "(tcp port 22 or icmp) and (host ${gw} or host 10.1.123.100 or host ${uplinkgw})" 2>&1 | sudo tee /tmp/host-tcpdump.txt > /dev/null &
  lxc exec micro01 -- timeout 8 tcpdump -ni any -c 60 "udp port 6081 or tcp port 22 or icmp" > /tmp/micro01-tcpdump.txt 2>&1 &
  sleep 2
  lxc exec micro01 -- lxc exec c1 -- timeout 5 bash -c "grep -qm1 ^SSH- < /dev/tcp/${gw}/22" && echo "c1->gateway:22 OK" || echo "c1->gateway:22 FAILED"
  wait
  echo "--- host tcpdump"
  cat /tmp/host-tcpdump.txt
  echo "--- micro01 tcpdump"
  cat /tmp/micro01-tcpdump.txt

  set -e
  echo "::endgroup::"
}

test_upgrade() {
  reset_systems 4 3 5

  # Clear out the debug binaries as we want to test upgrades from published versions.
  for i in $(seq -f "%02g" 1 4) ; do
    name="micro${i}"
    if [ -n "${MICROCLOUD_DEBUG_PATH}" ] || [ -n "${MICROCLOUDD_DEBUG_PATH}" ]; then
      lxc exec "${name}" -- rm -f /var/snap/microcloud/common/microcloud.debug || true
      lxc exec "${name}" -- rm -f /var/snap/microcloud/common/microcloudd.debug || true
      lxc exec "${name}" -- systemctl restart snap.microcloud.daemon || true
    fi

    if [ -n "${LXD_DEBUG_PATH}" ]; then
      lxc exec "${name}" -- rm -f /var/snap/lxd/common/lxd.debug
      lxc exec "${name}" -- systemctl reload snap.lxd.daemon || true
      lxc exec "${name}" -- lxd waitready
    fi
  done

  # Perform upgrade test from MicroCloud 2 to 3.
  if [ "${MICROCLOUD_SNAP_CHANNEL}" = "2/candidate" ]; then
    # Use the edge channels to catch issues early in the release process.
    # Can be moved back to "tentacle/edge" once https://bugs.launchpad.net/ubuntu/+source/ceph/+bug/2166817 is fixed and propagated in the LXD snap.
    microceph_target="tentacle/stable"
    microovn_target="26.03/edge"
    lxd_target="6/edge"
    microcloud_target="3/edge"

    addr=$(lxc ls micro01 -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')

    ceph_cluster_subnet_prefix="10.0.1"
    ceph_cluster_subnet_iface="enp7s0"
    ceph_public_subnet_prefix="10.0.2"
    ceph_public_subnet_iface="enp8s0"
    ovn_underlay_subnet_prefix="10.0.3"
    ovn_underlay_subnet_iface="enp9s0"
    set_cluster_subnet 4  "${ceph_cluster_subnet_iface}" "${ceph_cluster_subnet_prefix}"
    set_cluster_subnet 4  "${ceph_public_subnet_iface}" "${ceph_public_subnet_prefix}"
    set_cluster_subnet 4  "${ovn_underlay_subnet_iface}" "${ovn_underlay_subnet_prefix}"

    # Use the MicroCloud version 2 preseed format with all the features available at the time of release:
    # - Local and remote storage
    # - Wiping disks
    # - Encrypting disks
    # - OVN uplink interface, DNS servers, disaggregated
    # - CephFS
    # - Ceph network disaggregated
    preseed="lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.2
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true
- name: micro02
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.3
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true
- name: micro03
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.4
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true

ovn:
  ipv4_gateway: 10.1.123.1/24
  ipv4_range: 10.1.123.100-10.1.123.254
  ipv6_gateway: fd42:1:1234:1234::1/64
  dns_servers: 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1

ceph:
  internal_network: ${ceph_cluster_subnet_prefix}.0/24
  public_network: ${ceph_public_subnet_prefix}.0/24
  cephfs: true"

    if [ "${BASE_OS}" = "22.04" ]; then
      # On 22.04 machines we cannot use dm-crypt.
      # Therefore negate the instruction to encrypt the Ceph OSD disks.
      preseed="$(echo "${preseed}" | yq -e '.systems.[].storage.ceph.[].encrypt = false')"

      # On 22.04 machines we need to ensure ZFS tooling is present for LXD 6.
      for m in micro01 micro02 micro03 micro04; do
        retry lxc exec "${m}" -- apt-get install --no-install-recommends -y zfsutils-linux
        lxc exec "${m}" -- snap set lxd zfs.external=true
        lxc exec "${m}" -- snap restart lxd
      done
    fi

    # Deploy a version 2 MicroCloud and launch some instances.
    lxc exec micro02 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro03 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro02 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q
    lxc exec micro03 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM LAUNCH TEST"
    else
      lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 v1 --vm
      lxc exec micro01 -- sh -c "$(declare -f waitInstanceReady); waitInstanceReady v1"
      v1_boot_id=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
    fi
    lxc exec micro01 -- lxc launch ubuntu-minimal-daily:24.04 c1
    lxc exec micro01 -- sh -c "$(declare -f waitInstanceReady); waitInstanceReady c1"
    c1_boot_id="$(lxc exec micro01 -- lxc exec c1 -- cat /proc/sys/kernel/random/boot_id)"
    debug_host_ssh "before-upgrade"

    # First upgrade MicroCeph.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microceph --channel "${microceph_target}"
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "MicroCeph member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- microceph cluster list | grep "${m}" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    # Wait for MicroCeph to stabilize before proceeding.
    lxc exec micro01 -- microceph waitready --storage --timeout 300

    disks_encrypted="1"
    disks_encrypted_list="disk2"

    # On 22.04 machines the check should indicate no Ceph OSD encryption.
    if [ "${BASE_OS}" = "22.04" ]; then
      disks_encrypted="0"
      disks_encrypted_list=""
    fi

    for m in micro01 micro02 micro03; do
      validate_system_microceph "${m}" 1 "${disks_encrypted}" "${ceph_cluster_subnet_prefix}.0/24" "${ceph_public_subnet_prefix}.0/24" disk2 "${disks_encrypted_list}"
    done

    # Second upgrade MicroOVN.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microovn --channel "${microovn_target}"

      # Before proceeding with the next member, wait for ${m} to stabilize.
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "MicroOVN member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- microovn cluster list -f json | jq -r ".[] | select(.name == \"${m}\") | .status" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    for m in micro01 micro02 micro03; do
      validate_system_microovn ${m} "${ovn_underlay_subnet_prefix}"
    done

    # Third upgrade LXD.
    for m in micro01 micro02 micro03; do
      # Upgrade them in parallel as the refresh waits for the others to update too.
      lxc exec "${m}" -- snap refresh lxd --channel "${lxd_target}" &
    done
    wait

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "LXD member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" -- lxc cluster list -f json | jq -r ".[] | select(.server_name == \"${m}\") | .status" | grep -q Online; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    # Test LXD only after all the cluster members have stabilized after upgrade.
    for m in micro01 micro02 micro03; do
      SKIP_CEPH_RBD_FEATURES="1" validate_system_lxd "${m}" 3 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
    done

    # Fourth upgrade MicroCloud.
    for m in micro01 micro02 micro03; do
      lxc exec "${m}" -- snap refresh microcloud --channel "${microcloud_target}"
      set_debug_binaries "${m}"
    done

    for m in micro01 micro02 micro03; do
      retries=0
      while true; do
        if [ "${retries}" -gt 60 ]; then
          echo "MicroCloud member ${m} failed to come up after upgrade"
          exit 1
        fi

        if lxc exec "${m}" --env TEST_CONSOLE=0 -- microcloud cluster list -f json | jq -r ".[] | select(.name == \"${m}\") | .status" | grep -q ONLINE; then
          break
        fi

        sleep 1
        retries=$((retries+1))
      done
    done

    # Check if the workload survived the upgrade.
    # Perform some pings to check the network.
    gateway="$(lxc network get lxdbr0 ipv4.address | cut -d/ -f1)"
    debug_host_ssh "after-upgrade"
    if [ "${SKIP_VM_LAUNCH}" = "1" ]; then
      echo "::warning::SKIPPING VM TESTS"
    else
      lxc exec micro01 -- lxc exec v1 -- timeout 5 bash -cex "grep -qm1 ^SSH- < /dev/tcp/${gateway}/22"

      # Compare the boot ids to verify the instances haven't been restarted.
      v1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec v1 -- cat /proc/sys/kernel/random/boot_id)
      [ "${v1_boot_id}" = "${v1_boot_id_after_upgrade}" ]
    fi
    lxc exec micro01 -- lxc exec c1 -- timeout 5 bash -cex "grep -qm1 ^SSH- < /dev/tcp/${gateway}/22"
    c1_boot_id_after_upgrade=$(lxc exec micro01 -- lxc exec c1 -- cat /proc/sys/kernel/random/boot_id)
    [ "${c1_boot_id}" = "${c1_boot_id_after_upgrade}" ]

    # Upgrade micro04.
    lxc exec micro04 -- snap refresh microceph --channel "${microceph_target}"
    lxc exec micro04 -- snap refresh microovn --channel "${microovn_target}"
    # Workaround to prevent an error during refresh (https://github.com/canonical/concierge/issues/20).
    # Run configure hook of "lxd" snap if present (run hook "configure": error: cannot communicate with server: Post "http://localhost/v2/snapctl": dial unix /run/snapd-snap.socket: connect: no such file or directory)
    lxc exec micro04 -- snap stop lxd
    lxc exec micro04 -- snap refresh lxd --channel "${lxd_target}"
    lxc exec micro04 -- snap refresh microcloud --channel "${microcloud_target}"
    set_debug_binaries "micro04"

    # Join micro04 to the old cluster using the MicroCloud 2 preseed format.
    preseed="lookup_subnet: ${addr}/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro04
  ovn_uplink_interface: enp6s0
  ovn_underlay_ip: 10.0.3.5
  storage:
    local:
      path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk1
      wipe: true
    ceph:
      - path: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_disk2
        wipe: true
        encrypt: true"

    if [ "${BASE_OS}" = "22.04" ]; then
      preseed="$(echo "${preseed}" | yq -e '.systems.[].storage.ceph.[].encrypt = false')"
    fi

    lxc exec micro04 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed" &
    lxc exec micro01 --env TEST_CONSOLE=0 -- sh -c 'microcloud preseed > out' <<< "$preseed"

    lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
    lxc exec micro04 -- tail -2 out | head -1 | grep "Successfully joined the MicroCloud cluster and closing the session" -q

    validate_system_microceph micro04 1 "${disks_encrypted}" "${ceph_cluster_subnet_prefix}.0/24" "${ceph_public_subnet_prefix}.0/24" disk2 "${disks_encrypted_list}"
    validate_system_microovn micro04 "${ovn_underlay_subnet_prefix}"
    SKIP_CEPH_RBD_FEATURES="1" validate_system_lxd micro04 4 disk1 1 1 enp6s0 10.1.123.1/24 10.1.123.100-10.1.123.254 fd42:1:1234:1234::1/64 10.1.123.1,8.8.8.8,fd42:1:1234:1234::1
    lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster list -f json | jq -r '.[] | select(.name == "micro04") | .status' | grep -q ONLINE
  fi
}
