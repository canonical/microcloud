#!/bin/bash

test_e2e() {
  # Reset the systems and install microovn and microceph.
  reset_systems 3 3 2

  microcloud_internal_net_addr="$(ip_config_to_netaddr lxdbr0)"

  echo "Creating a MicroCloud with ZFS and Ceph storage, and OVN network"
  unset SKIP_SERVICE
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
  export IPV6_SUBNET="fd42:1:1234:1234::1/64"
  export DNS_ADDRESSES="10.1.123.1,8.8.8.8"
  export OVN_UNDERLAY_NETWORK="no"
  join_session init micro01 micro02 micro03

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q
  for m in micro01 micro02 micro03 ; do
    validate_system_lxd "${m}" 3 disk1 3 1 "${OVN_FILTER}" "${IPV4_SUBNET}" "${IPV4_START}"-"${IPV4_END}" "${IPV6_SUBNET}" "${DNS_ADDRESSES}"
    validate_system_microceph "${m}" 1 disk2
    validate_system_microovn "${m}"
  done

  # e2e test
  cd e2e || exit 1
  TOKEN="$(lxc exec micro01 -- lxc config trust add --quiet --name e2e)"
  MICRO01_IP="$(lxc list -f csv -c 4 micro01 | awk '/enp5s0/ {print $1}')"
  REMOTE_ADDRESS="https://${MICRO01_IP}:8443"
  if lxc remote list -f csv | cut -d',' -f1 | grep -qxF mc; then
    lxc remote remove mc
  fi
  lxc remote add mc "${REMOTE_ADDRESS}" --token "${TOKEN}"

  echo "==> Run microcloud E2E test"

  e2e_testbed_file="$(pwd)/testbed.yaml"
  cat > testbed.yaml <<-EOF
lxd:
  project: microcloud-test
EOF

  export REMOTE="mc"
  export E2E_TESTBED_FILE=${e2e_testbed_file}
  go run github.com/onsi/ginkgo/v2/ginkgo -v --junit-report=report.xml .
}
