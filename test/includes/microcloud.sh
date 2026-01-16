#!/bin/bash

# unset_interactive_vars: Unsets all variables related to the test console.
unset_interactive_vars() {
  unset SKIP_LOOKUP LOOKUP_IFACE SKIP_SERVICE EXPECT_PEERS PEERS_FILTER REUSE_EXISTING REUSE_EXISTING_COUNT \
    SETUP_ZFS ZFS_FILTER ZFS_WIPE \
    SETUP_CEPH CEPH_FILTER CEPH_WIPE CEPH_ENCRYPT SETUP_CEPHFS CEPH_CLUSTER_NETWORK CEPH_PUBLIC_NETWORK \
    PROCEED_WITH_NO_OVERLAY_NETWORKING SETUP_OVN_EXPLICIT SETUP_OVN_IMPLICIT OVN_UNDERLAY_NETWORK OVN_UNDERLAY_FILTER OVN_WARNING OVN_FILTER IPV4_SUBNET IPV4_START IPV4_END DNS_ADDRESSES IPV6_SUBNET \
    REPLACE_PROFILE CEPH_RETRY_HA MULTI_NODE
}

# microcloud_interactive: generates text that is being passed to `TEST_CONSOLE=1 microcloud *`
# to simulate terminal input to the interactive CLI.
# The lines that are output are based on the values passed to the listed environment variables.
# Any unset variables will be omitted.
# The first argument is the microcloud command you want to run interactively.
# The second argument is the system on which the command gets executed.
microcloud_interactive() {
  enable_xtrace=0

  if set -o | grep -q "xtrace.*on" ; then
    enable_xtrace=1
    set +x
  fi

  MULTI_NODE=${MULTI_NODE:-}                     # (yes/no) whether to set up multiple nodes
  SKIP_LOOKUP=${SKIP_LOOKUP:-}                   # whether or not to skip the whole lookup block in the interactive command list.
  LOOKUP_IFACE=${LOOKUP_IFACE:-}                 # filter string for the lookup interface table.
  SKIP_SERVICE=${SKIP_SERVICE:-}                 # (yes/no) input to skip any missing services. Should be unset if all services are installed.
  EXPECT_PEERS=${EXPECT_PEERS:-}                 # wait for this number of systems to be available to join the cluster.
  PEERS_FILTER=${PEERS_FILTER:-}                 # filter string for the particular peer to init/add
  REUSE_EXISTING=${REUSE_EXISTING:-}              # (yes/no) incorporate an existing clustered service.
  REUSE_EXISTING_COUNT=${REUSE_EXISTING_COUNT:-0} # (number) number of existing clusters to incorporate.
  SETUP_ZFS=${SETUP_ZFS:-}                       # (yes/no) input for initiating ZFS storage pool setup.
  ZFS_FILTER=${ZFS_FILTER:-}                     # filter string for ZFS disks.
  ZFS_WIPE=${ZFS_WIPE:-}                         # (yes/no) to wipe all disks.
  SETUP_CEPH=${SETUP_CEPH:-}                     # (yes/no) input for initiating Ceph storage pool setup.
  SKIP_CEPH_DISKS=${SKIP_CEPH_DISKS:-}           # (yes/no) input to skip adding additional Ceph disks and only reuse the existing cluster and its disks.
  SETUP_CEPHFS=${SETUP_CEPHFS:-}                 # (yes/no) input for initialising CephFS storage pool setup.
  CEPH_FILTER=${CEPH_FILTER:-}                   # filter string for CEPH disks.
  CEPH_WIPE=${CEPH_WIPE:-}                       # (yes/no) to wipe all disks.
  CEPH_RETRY_HA=${CEPH_RETRY_HA:-}                     # (yes/no) input for warning setup is not HA.
  CEPH_ENCRYPT=${CEPH_ENCRYPT:-}                  # (yes/no) to encrypt all disks.
  CEPH_CLUSTER_NETWORK=${CEPH_CLUSTER_NETWORK:-} # (default: MicroCloud internal subnet) input for setting up a cluster network.
  CEPH_PUBLIC_NETWORK=${CEPH_PUBLIC_NETWORK:-}   # (default: MicroCloud internal subnet or Ceph internal network if specified previously) input for setting up a public network.
  PROCEED_WITH_NO_OVERLAY_NETWORKING=${PROCEED_WITH_NO_OVERLAY_NETWORKING:-} # (yes/no) input for proceeding without overlay networking.
  SETUP_OVN_EXPLICIT=${SETUP_OVN_EXPLICIT:-}      # (yes/no) input for explicitly initiating OVN network setup during bootstrap.
  SETUP_OVN_IMPLICIT=${SETUP_OVN_IMPLICIT:-}      # (yes/no) input for implicitly initiating OVN network setup during join as it doesn't anymore ask to configure distributed networking.
  OVN_WARNING=${OVN_WARNING:-}                    # (yes/no) input for warning about eligible interface detection.
  OVN_FILTER=${OVN_FILTER:-}                      # filter string for OVN interfaces.
  IPV4_SUBNET=${IPV4_SUBNET:-}                    # OVN ipv4 gateway subnet.
  IPV4_START=${IPV4_START:-}                      # OVN ipv4 range start.
  IPV4_END=${IPV4_END:-}                          # OVN ipv4 range end.
  DNS_ADDRESSES=${DNS_ADDRESSES:-}                # OVN custom DNS addresses.
  OVN_UNDERLAY_NETWORK=${OVN_UNDERLAY_NETWORK:-}  # (yes/no) set up a custom OVN underlay network.
  OVN_UNDERLAY_FILTER=${OVN_UNDERLAY_FILTER:-}    # filter string for OVN underlay interfaces.
  IPV6_SUBNET=${IPV6_SUBNET:-}                    # OVN ipv6 range.
  REPLACE_PROFILE="${REPLACE_PROFILE:-}"          # Replace default profile config and devices.

  setup=""
  if [ -n "${MULTI_NODE}" ]; then
    setup="
${MULTI_NODE}                                           # lookup multiple nodes
$([ -n "${LOOKUP_IFACE}" ] && printf "table:filter %s" "${LOOKUP_IFACE}")          # filter the lookup interface
$([ -n "${LOOKUP_IFACE}" ] && printf "table:select")          # select the interface
$([ -n "${LOOKUP_IFACE}" ] && printf -- "table:done")
$(true)
"
  fi

  if ! [ "${SKIP_LOOKUP}" = 1 ]; then
    setup="${setup}
$([ "${SKIP_SERVICE}" = "yes" ] && printf "%s" "${SKIP_SERVICE}")  # skip MicroOVN/MicroCeph (yes/no)
table:expect ${EXPECT_PEERS}                                      # wait until the systems show up
$([ -n "${PEERS_FILTER}" ] && printf "table:filter %s" "${PEERS_FILTER}")          # filter discovered peers
table:select-all                                                  # select all the systems
table:done
$(true)                                                 # workaround for set -e
"
  fi

if [ -n "${REUSE_EXISTING}" ]; then
  for i in $(seq 1 "${REUSE_EXISTING_COUNT}") ; do
    setup=$(cat << EOF
${setup}
${REUSE_EXISTING}
EOF
)
  done
fi

if [ -n "${SETUP_ZFS}" ]; then
  setup="${setup}
${SETUP_ZFS}                                            # add local disks (yes/no)
$([ "${SETUP_ZFS}" = "yes" ] && printf "table:wait 300ms")    # wait for the table to populate
$([ -n "${ZFS_FILTER}" ] && printf "table:filter %s" "${ZFS_FILTER}")          # filter zfs disks
$([ "${SETUP_ZFS}" = "yes" ] && printf "table:select-all")    # select all disk matching the filter
$([ "${SETUP_ZFS}" = "yes" ] && printf -- "table:done" )
$([ "${ZFS_WIPE}"  = "yes" ] && printf "table:select-all")    # wipe all disks
$([ "${SETUP_ZFS}" = "yes" ] && printf -- "table:done")
$(true)                                                 # workaround for set -e
"
fi

if [ -n "${SETUP_CEPH}" ]; then
  setup="${setup}
${SETUP_CEPH}                                           # add remote disks (yes/no)
"
  if [ "${SKIP_CEPH_DISKS}" != "yes" ]; then
    setup="${setup}
$([ "${SETUP_CEPH}" = "yes" ] && printf "table:wait 300ms")       # wait for the table to populate
$([ -n "${CEPH_FILTER}" ] && printf "table:filter %s" "${CEPH_FILTER}")          # filter ceph disks
$([ "${SETUP_CEPH}" = "yes" ] && printf "table:select-all")       # select all disk matching the filter
$([ "${SETUP_CEPH}" = "yes" ] && printf -- "table:done")
$([ "${CEPH_WIPE}"  = "yes" ] && printf "table:select-all")       # wipe all disks
$([ "${SETUP_CEPH}" = "yes" ] && printf -- "table:done")
$([ "${SETUP_CEPH}" = "yes" ] && printf "%s" "${CEPH_RETRY_HA}" ) # allow ceph setup without 3 systems supplying disks.
$(true)                                                           # workaround for set -e
"
  fi

  setup="${setup}
${CEPH_ENCRYPT}                                                          # encrypt disks? (yes/no)
${SETUP_CEPHFS}
$([ "${SETUP_CEPH}" = "yes" ] && printf "%s" "${CEPH_CLUSTER_NETWORK}" ) # set ceph cluster network
$([ "${SETUP_CEPH}" = "yes" ] && printf "%s" "${CEPH_PUBLIC_NETWORK}" )  # set ceph public network
$(true)                                                                  # workaround for set -e
"
fi

if [ -n "${PROCEED_WITH_NO_OVERLAY_NETWORKING}" ]; then
  setup="${setup}
${PROCEED_WITH_NO_OVERLAY_NETWORKING}                   # agree to proceed without overlay networking (neither FAN nor OVN networking) (yes/no)
$(true)                                                 # workaround for set -e
"
fi

if [ -n "${OVN_WARNING}" ] ; then
  setup="${setup}
${OVN_WARNING}                                         # continue with some peers missing an interface? (yes/no)
$(true)                                                 # workaround for set -e
"
fi

if [ -n "${SETUP_OVN_EXPLICIT}" ] || [ -n "${SETUP_OVN_IMPLICIT}" ]; then
  if [ -n "${SETUP_OVN_EXPLICIT}" ]; then
      setup="${setup}
${SETUP_OVN_EXPLICIT}  # agree to explicitly setup OVN
"
  fi

  setup="${setup}
$([ "${SETUP_OVN_EXPLICIT}" = "yes" ] || [ "${SETUP_OVN_IMPLICIT}" = "yes" ] && printf "table:wait 300ms")   # wait for the table to populate
$([ -n "${OVN_FILTER}" ] && printf "table:filter %s" "${OVN_FILTER}")          # filter interfaces
$([ "${SETUP_OVN_EXPLICIT}" = "yes" ] || [ "${SETUP_OVN_IMPLICIT}" = "yes" ] && printf "table:select-all")   # select all interfaces matching the filter
$([ "${SETUP_OVN_EXPLICIT}" = "yes" ] || [ "${SETUP_OVN_IMPLICIT}" = "yes" ] && printf -- "table:done")
${IPV4_SUBNET}                                         # setup ipv4/ipv6 gateways and ranges
${IPV4_START}
${IPV4_END}
${IPV6_SUBNET}
${DNS_ADDRESSES}
$(true)                                                 # workaround for set -e
"

  if [ -n "${OVN_UNDERLAY_NETWORK}" ]; then
    setup="${setup}
${OVN_UNDERLAY_NETWORK}
$([ "${OVN_UNDERLAY_NETWORK}" = "yes" ] && printf "table:wait 300ms")
$([ -n "${OVN_UNDERLAY_FILTER}" ] && printf "table:filter %s" "${OVN_UNDERLAY_FILTER}")
$([ "${OVN_UNDERLAY_NETWORK}" = "yes" ] && printf "table:select-all")
$([ "${OVN_UNDERLAY_NETWORK}" = "yes" ] && printf -- "table:done")
$(true)                                                 # workaround for set -e
"
  fi
fi

if [ -n "${REPLACE_PROFILE}" ] ; then
  setup="${setup}
${REPLACE_PROFILE}
$(true)                                                 # workaround for set -e
"
fi

  # clear comments and empty lines.
  setup="$(echo "${setup}" | sed '/^\s*#/d; s/\s*#.*//; /^$/d' | tee /dev/stderr)"
  if [ ${enable_xtrace} = 1 ]; then
    set -x
  fi

  # append the session timeout if applicable.
  args=""
  if [ "${1}" = "init" ] || [ "${1}" = "add" ]; then
    args="--session-timeout=60"
  fi

  echo "${setup}" | lxc exec "${2}" -- sh -c "microcloud ${1} ${args} > out 2>&1 3>debug"
}

# join_session: Set up a MicroCloud join session.
# Joiners are spawned as child processes, and are forcibly killed if still running after the initiator returns.
# Arg 1: initiator command to run (init/add)
# Arg 2: initiator name
# Remaining args: names of joiners
join_session(){
  enable_xtrace=0
  mode="${1}"
  initiator="${2}"
  shift 2
  joiners="${*}"

  if set -o | grep -q "xtrace.*on" ; then
    enable_xtrace=1
    set +x
  fi

  LOOKUP_IFACE=${LOOKUP_IFACE:-} # filter string for the lookup interface table.

  # If we are adding a node, only the joiners will need to supply an address, so pick the first one.
  if [ "${mode}" = "add" ] && [ -z "${LOOKUP_IFACE}" ] ; then
    LOOKUP_IFACE="enp5s0"
  fi

  # Select the first usable address and enter the passphrase.
  setup="$([ -n "${LOOKUP_IFACE}" ] && printf "table:filter %s" "${LOOKUP_IFACE}")          # filter the lookup interface
$([ -n "${LOOKUP_IFACE}" ] && printf "table:select")          # select the interface
$([ -n "${LOOKUP_IFACE}" ] && printf -- "table:done")
a a a a                                 # the test passphrase
$(printf "ctrl:m")						# autocomplete model requires carriage return
$(true)                                 # workaround for set -e
"

  # Clear comments and empty lines.
  setup="$(echo "${setup}" | sed '/^\s*#/d; s/\s*#.*//; /^$/d' | tee /dev/stderr)"
  if [ ${enable_xtrace} = 1 ]; then
    set -x
  fi

  # Spawn joiner child processes, and capture the exit code.
  for member in ${joiners}; do
    tmux new-session -d -s "${member}" lxc exec "${member}" -- sh -c "tee in | (microcloud join 2>&1 3>debug; echo \$? > code) | tee out"

    retries=0

    # Wait until CLI is in testing mode.
    while [[ ! "$(tmux capture-pane -t "${member}" -pS -)" =~ "MicroCloud CLI is in testing mode" ]]; do
      if [ "${retries}" -gt 30 ]; then
        echo "Failed waiting for test console on ${member} to become ready"
        exit 1
      fi

      retries="$((retries+1))"
      sleep 1
    done

    # Send the interactive test inputs, a newline and EOF.
    tmux send-keys -t "${member}" "${setup}" Enter C-d
  done

  # Set up microcloud with the initiator.
  microcloud_interactive "${mode}" "${initiator}"
  code="$?"

  # Kill the tmux sessions if they are still running.
  # The '#S' filter returns only the session's names.
  # Suppress errors in case the list of sessions is empty.
  for session in $(tmux list-sessions -F '#S' 2>/dev/null); do
    tmux kill-session -t "${session}"
  done

  for joiner in ${joiners} ; do
    [ "$(lxc file pull "${joiner}"/root/code -)" = "${code}" ]
  done

  return "${code}"
}

# set_debug_binaries: Adds {app}.debug binaries if the corresponding {APP}_DEBUG_PATH environment variable is set.
set_debug_binaries() {
  name="${1}"

  if [ -n "${MICROOVN_SNAP_PATH}" ]; then
    echo "==> Add local build of MicroOVN snap."
    lxc file push --quiet "${MICROOVN_SNAP_PATH}" "${name}/root/microovn.snap"
    lxc exec "${name}" -- snap install --dangerous "/root/microovn.snap"
  fi

  if [ -n "${MICROCEPH_SNAP_PATH}" ]; then
    echo "==> Add local build of MicroCeph snap."
    lxc file push --quiet "${MICROCEPH_SNAP_PATH}" "${name}/root/microceph.snap"
    lxc exec "${name}" -- snap install --dangerous "/root/microceph.snap"
  fi

  if [ -n "${MICROCLOUD_DEBUG_PATH}" ] && [ -n "${MICROCLOUDD_DEBUG_PATH}" ]; then
    echo "==> Add debug binaries for MicroCloud."

    # Before injecting the binaries ensure the daemon is stopped.
    # As the other cluster members might already have the new version, this member's version
    # might still be behind after the snap refresh so the daemon enters a restart loop yielding "This node's version is behind, please upgrade."
    # Therefore shut it down to exit the loop and ensure it can cleanly come up (with the right version) after injecting the binaries.
    lxc exec "${name}" -- systemctl stop snap.microcloud.daemon || true

    lxc exec "${name}" -- rm -f /var/snap/microcloud/common/microcloudd.debug /var/snap/microcloud/common/microcloud.debug

    lxc file push --quiet "${MICROCLOUDD_DEBUG_PATH}" "${name}"/var/snap/microcloud/common/microcloudd.debug --mode 0755
    lxc file push --quiet "${MICROCLOUD_DEBUG_PATH}" "${name}"/var/snap/microcloud/common/microcloud.debug --mode 0755

    lxc exec "${name}" -- systemctl start snap.microcloud.daemon || true
  fi

  if [ -n "${LXD_DEBUG_PATH}" ]; then
    echo "==> Add a debug binary for LXD."
    lxc exec "${name}" -- rm -f /var/snap/lxd/common/lxd.debug
    lxc file push --quiet "${LXD_DEBUG_PATH}" "${name}"/var/snap/lxd/common/lxd.debug
    lxc exec "${name}" -- systemctl reload snap.lxd.daemon || true
    lxc exec "${name}" -- lxd waitready
  fi
}

# set_remote: Adds and switches to the remote for the MicroCloud node with the given name.
set_remote() {
  remote="${1}"
  name="${2}"
  client="${3}"

  lxc remote switch local

  if lxc remote list -f csv | cut -d',' -f1 | grep -qwF "${remote}" ; then
    lxc remote remove "${remote}"
  fi

  token="$(lxc exec "${name}" -- lxc config trust add --name "${client}" --quiet)"

  # Suppress the confirmation as it's noisy.
  lxc remote add "${remote}" "${token}" > /dev/null 2>&1
  lxc remote switch "${remote}"
}

# validate_system_microceph: Ensures the node with the given name has correctly set up MicroCeph with the given resources.
validate_system_microceph() {
    name=${1}
    shift 1

    cephfs=0
    if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
      cephfs="${1}"
      shift 1
    fi

    encrypt=0
    if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
      encrypt="${1}"
      shift 1
    fi

    cluster_ceph_subnet=""
    if echo "${1:-}" | grep -Pq '^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$'; then
      cluster_ceph_subnet="${1}"
      shift 1
    fi

    public_ceph_subnet=""
    if echo "${1:-}" | grep -Pq '^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]|[1-2][0-9]|3[0-2])$'; then
      public_ceph_subnet="${1}"
      shift 1
    fi

    if [ "${encrypt}" = 1 ]; then
      local disks_to_encrypt="${1}"
      shift 1
      validate_ceph_encrypt "${name}" "${disks_to_encrypt}"
    fi

    disks="${*}"

    echo "==> ${name} Validating MicroCeph. Using disks: {${disks}}, Using CephFS: {${cephfs}}, Cluster Ceph Subnet: {${cluster_ceph_subnet}}, Public Ceph Subnet: {${public_ceph_subnet}}, Encrypt: {${encrypt}}"

    lxc remote switch local
    lxc exec "${name}" -- sh -ceu "
      microceph cluster list | grep -q ${name}

      count=0
      for disk in ${disks} ; do
        ceph_disks=\$(microceph cluster sql \"select name, path from disks join core_cluster_members on core_cluster_members.id = disks.member_id where path like '%\${disk}' and name = '${name}'\")
        echo \"\${ceph_disks}\" | grep -q \"/dev/disk/by-id/scsi-.*_lxd_\${disk}\"
        count=\$((count + 1))
      done

     query='{\"query\": \"select count(*) from disks join core_cluster_members on core_cluster_members.id = disks.member_id where core_cluster_members.name = \\\"${name}\\\"\"}'
     count_disks=\$(curl --unix-socket /var/snap/microceph/common/state/control.socket ./core/internal/sql -X POST -d \"\${query}\" -s)
     echo \"\${count_disks}\" | jq '.status_code' | grep -q 200
     echo \"\${count_disks}\" | jq '.metadata .Results[0] .rows[0][0]' | grep -q \${count}
    "

    if [ "${cephfs}" = 1 ]; then
      lxc exec "${name}" -- sh -ceu "
        microceph.ceph fs get lxd_cephfs
        microceph.ceph osd pool get lxd_cephfs_meta size
        microceph.ceph osd pool get lxd_cephfs_data size
      " > /dev/null
    fi

    if [ -n "${cluster_ceph_subnet}" ]; then
      lxc exec "${name}" -- sh -ceu "
        microceph.ceph config show osd.1 cluster_network | grep -q ${cluster_ceph_subnet}
      " > /dev/null
    fi

    if [ -n "${public_ceph_subnet}" ]; then
      lxc exec "${name}" -- sh -ceu "
        microceph.ceph config show osd.1 public_network | grep -q ${public_ceph_subnet}
      " > /dev/null
    fi
}

# validate_ceph_encrypt: Ensures the node with the given name has an encrypted disk.
validate_ceph_encrypt() {
  name=${1}
  shift 1
  IFS=',' read -ra disks_list <<< "$1"

  disks="${disks_list[*]}"
  echo "==> ${name} Validating Ceph encryption. Using disks: {${disks}}"

  lxc remote switch local
  lxc exec "${name}" -- sh -ceu "
    for disk in ${disks} ; do
      ceph_disks=\$(microceph cluster sql \"select path from disks join core_cluster_members on core_cluster_members.id = disks.member_id where path like '%\${disk}' and name = '${name}'\")
      disks_paths=\$(echo \"\${ceph_disks}\" | grep -o /dev/disk/by-id/scsi-.*_lxd_\${disk})
      devname=\$(udevadm info --query=property --name=\"\$disks_paths\" | grep '^DEVNAME=' | cut -d'=' -f2 | cut -d'/' -f3)
      result=\$(lsblk -l -o name,fstype | grep -E \"^\b\$devname\b.*crypto_LUKS\")

      if [ -n \"\$result\" ]; then
        echo \"\$devname is encrypted.\"
      else
        echo \"No corresponding encrypted /dev/sdX found.\"
        exit 1
      fi
    done
  "
}

# validate_system_microovn: Ensures the node with the given name has correctly set up MicroOVN with the given resources.
validate_system_microovn() {
    name=${1}
    shift 1

    echo "==> ${name} Validating MicroOVN"

    lxc remote switch local
    lxc exec "${name}" -- microovn cluster list | grep -q "${name}"

    ovn_underlay_subnet_prefix=""
    if [ $# -gt 0 ] && echo "${1}" | grep -Pq '^([0-9]{1,3}\.){2}[0-9]{1,3}$'; then
      ovn_underlay_subnet_prefix="${1}"
      shift 1
    fi

    # Instances are named micro01, micro02, etc.
    # We need to extract the instance number to check the OVN Geneve tunnel IP.
    instance_idx=$(echo "${name}" | grep -oE '[0-9]+$')
    underlay_ip_idx=$((instance_idx + 1))
    if [ -n "${ovn_underlay_subnet_prefix}" ]; then
      lxc exec "${name}" -- sh -ceu "
        microovn.ovn-sbctl --columns=ip,type find Encap type=geneve | grep -q ${ovn_underlay_subnet_prefix}.${underlay_ip_idx}
      " > /dev/null
    fi
}
# validate_system_lxd_zfs: Ensures the node with the given name has the given disk set up for ZFS storage.
validate_system_lxd_zfs() {
  name=${1}
  local_disk=${2:-}
  echo "    ${name} Validating ZFS storage"
  [ "$(lxc config get storage.backups_volume --target "${name}")" = "local/backups" ]
  [ "$(lxc config get storage.images_volume  --target "${name}")" = "local/images"  ]

  cfg="$(lxc storage show local)"
  grep -q "config: {}" <<< "${cfg}"
  grep -q "status: Created" <<< "${cfg}"

  cfg="$(lxc storage show local --target "${name}")"
  grep -q "source: local" <<< "${cfg}"
  grep -q "volatile.initial_source: .*${local_disk}" <<< "${cfg}"
  grep -q "zfs.pool_name: local" <<< "${cfg}"
  grep -q "driver: zfs" <<< "${cfg}"
  grep -q "status: Created" <<< "${cfg}"
  grep -q "/1.0/storage-pools/local/volumes/custom/backups?target=${name}" <<< "${cfg}"
  grep -q "/1.0/storage-pools/local/volumes/custom/images?target=${name}" <<< "${cfg}"
}

# validate_system_lxd_ceph: Ensures the node with the given name has ceph storage set up.
validate_system_lxd_ceph() {
  name=${1}
  cephfs=${2}
  echo "    ${name} Validating Ceph storage"
  cfg="$(lxc storage show remote)"
  grep -q "ceph.cluster_name: ceph" <<< "${cfg}"
  grep -q "ceph.osd.pg_num: \"32\"" <<< "${cfg}"
  grep -q "ceph.osd.pool_name: lxd_remote" <<< "${cfg}"
  grep -q "ceph.rbd.du: \"false\"" <<< "${cfg}"
  grep -q "ceph.rbd.features: layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten" <<< "${cfg}"
  grep -q "ceph.user.name: admin" <<< "${cfg}"
  grep -q "volatile.pool.pristine: \"true\"" <<< "${cfg}"
  grep -q "status: Created" <<< "${cfg}"
  grep -q "driver: ceph" <<< "${cfg}"

  cfg="$(lxc storage show remote --target "${name}")"
  grep -q "source: lxd_remote" <<< "${cfg}"
  grep -q "status: Created" <<< "${cfg}"

  if [ "${cephfs}" = 1 ]; then
    cfg="$(lxc storage show remote-fs)"
    grep -q "status: Created" <<< "${cfg}"
    grep -q "driver: cephfs" <<< "${cfg}"
    grep -q "cephfs.meta_pool: lxd_cephfs_meta" <<< "${cfg}"
    grep -q "cephfs.data_pool: lxd_cephfs_data" <<< "${cfg}"

    cfg=$(lxc storage show remote-fs --target "${name}")
    grep -q "source: lxd_cephfs" <<< "${cfg}"
    grep -q "status: Created" <<< "${cfg}"
  else
    ! lxc storage show remote-fs || true
  fi
}

# validate_system_lxd_ovn: Ensures the node with the given name and config has ovn network set up correctly.
validate_system_lxd_ovn() {
  name=${1}
  num_peers=${2}
  ovn_interface=${3:-}
  ipv4_gateway=${4:-}
  ipv4_ranges=${5:-}
  ipv6_gateway=${6:-}
  dns_namesersers=${7:-}

  echo "    ${name} Validating OVN network"

  num_conns=3
  if [ "${num_peers}" -lt "${num_conns}" ]; then
    num_conns="${num_peers}"
  fi

  [ "$(lxc config get network.ovn.northbound_connection --target "${name}" | sed -e 's/,/\n/g' | wc -l)" = "${num_conns}" ]

  # Make sure there's no empty addresses.
  ! lxc config get network.ovn.northbound_connection --target "${name}" | sed -e 's/,/\n/g' | grep -q '^ssl:$' || false
  ! lxc config get network.ovn.northbound_connection --target "${name}" | sed -e 's/,/\n/g' | grep -q '^ssl::' || false

  # Check that the created UPLINK network has the right DNS servers.
  if [ -n "${dns_namesersers}" ] ; then
    dns_addresses=$(lxc exec "local:${name}" -- lxc network get UPLINK dns.nameservers)
    if [ "${dns_addresses}" != "${dns_namesersers}" ] ; then
      echo "ERROR: UPLINK network has wrong DNS server addresses: ${dns_addresses}"
      return 1
    fi
  fi

  cfg="$(lxc network show UPLINK)"
  grep -q "status: Created" <<< "${cfg}"
  grep -q "type: physical" <<< "${cfg}"

  if [ -n "${ipv4_gateway}" ] ; then
    grep -qF "ipv4.gateway: ${ipv4_gateway}" <<< "${cfg}"
  fi

  if [ -n "${ipv4_ranges}" ] ; then
    grep -qF "ipv4.ovn.ranges: ${ipv4_ranges}" <<< "${cfg}"
  fi

  if [ -n "${ipv6_gateway}" ] ; then
    grep -qF "ipv6.gateway: ${ipv6_gateway}" <<< "${cfg}"
  fi

  lxc network show UPLINK --target "${name}" | grep -qF "parent: ${ovn_interface}"

  cfg="$(lxc network show default)"
  grep -q "status: Created" <<< "${cfg}"
  grep -q "type: ovn" <<< "${cfg}"
  grep -q "network: UPLINK" <<< "${cfg}"
}

# validate_system_lxd_fan: Ensures the node with the given name has the Ubuntu FAN network set up correctly.
validate_system_lxd_fan() {
  name=${1}
  echo "    ${name} Validating FAN network"
  cfg="$(lxc network show lxdfan0)"
  grep -q "status: Created" <<< "${cfg}"
  grep -q "type: bridge" <<< "${cfg}"
  grep -q "bridge.mode: fan" <<< "${cfg}"
}

# validate_system_lxd: Ensures the node with the given name has correctly set up LXD with the given resources.
validate_system_lxd() {
    name=${1}
    num_peers=${2}
    local_disk=${3:-}
    remote_disks=${4:-0}
    cephfs=${5:-0}
    ovn_interface=${6:-}
    ipv4_gateway=${7:-}
    ipv4_ranges=${8:-}
    ipv6_gateway=${9:-}
    dns_namesersers=${10:-}
    profile_pool=${11:-}

    echo "==> ${name} Validating LXD with ${num_peers} peers"
    echo "    ${name} Local Disk: {${local_disk}}, Remote Disks: {${remote_disks}}, OVN Iface: {${ovn_interface}}"
    echo "    ${name} IPv4 Gateway: {${ipv4_gateway}}, IPv4 Ranges: {${ipv4_ranges}}"
    echo "    ${name} IPv6 Gateway: {${ipv6_gateway}}"

    lxc remote switch local

    # Add the peer as a remote using the name test for the trust.
    set_remote microcloud-test "${name}" test

    # Ensure we are clustered and online.
    # Use the direct API reponse to not be affected by CLI changes.
    lxc cluster list -f json | jq -e '.[] | select(.server_name == "'"${name}"'" and .status == "Online")' >/dev/null
    [ "$(lxc cluster list -f csv | wc -l)" = "${num_peers}" ]

    # Check core config options
    lxd_address="$(lxc config get core.https_address)"
    if [ "${MICROCLOUD_SNAP_CHANNEL}" = "1/candidate" ]; then
      # There was a bug in MicroCloud 1 that set different addresses.
      # See https://github.com/canonical/microcloud/issues/214
      system_address="$(lxc ls local:"${name}" -f json -c4 | jq -r '.[0].state.network.enp5s0.addresses[] | select(.family == "inet") | .address')"
      [ "${lxd_address}" = "${system_address}:8443" ] || [ "${lxd_address}" = "[::]:8443" ]
    else
      [ "${lxd_address}" = "[::]:8443" ]
    fi

    has_microovn=0
    has_microceph=0

    # These look like errors so suppress them to avoid confusion.
    {
      { lxc exec local:"${name}" -- microovn cluster list > /dev/null && has_microovn=1; } || true
      { lxc exec local:"${name}" -- microceph cluster list > /dev/null && has_microceph=1; } || true
    } > /dev/null 2>&1

    if [ "${has_microovn}" = 1 ] && [ -n "${ovn_interface}" ] ; then
      validate_system_lxd_ovn "${name}" "${num_peers}" "${ovn_interface}" "${ipv4_gateway}" "${ipv4_ranges}" "${ipv6_gateway}" "${dns_namesersers}"
    else
      validate_system_lxd_fan "${name}"
    fi

    if [ -n "${local_disk}" ]; then
      validate_system_lxd_zfs "${name}" "${local_disk}"
    fi

    if [ "${has_microceph}" = 1 ] && [ "${remote_disks}" -gt 0 ] ; then
      validate_system_lxd_ceph "${name}" "${cephfs}"
    fi

    echo "    ${name} Validating Profiles"
    if [ -n "${profile_pool}" ]; then
      lxc profile device get default root pool | grep -q "${profile_pool}"
    elif [ "${has_microceph}" = 1 ] && [ "${remote_disks}" -gt 0 ] ; then
      lxc profile device get default root pool | grep -q "remote"
    elif [ -n "${local_disk}" ] ; then
      lxc profile device get default root pool | grep -q "local"
    else
      ! lxc profile device list default | grep -q "root" || false
    fi

    if [ "${has_microovn}" = 1 ] && [ -n "${ovn_interface}" ] ; then
      lxc profile device get default eth0 network | grep -q "default"
    else
      lxc profile device get default eth0 network | grep -q "lxdfan0"
    fi

    # Only check these if MicroCloud is at least > 2.1.0.
    # We cannot just check for the 2 track as the change which sets user.microcloud might be in the 2/edge (latest/edge) but not yet 2/candidate channel.
    # The sort command exits with 1 in case the versions are equal and exits with 0 in case the version of MicroCloud is bigger than 2.1.0.
    microcloud_version="$(lxc exec local:"${name}" --env TEST_CONSOLE=0 -- microcloud --version | cut -d' ' -f1)"
    if ! printf "%s\n2.1.0" "${microcloud_version}" | sort -C -V || [ "${MICROCLOUD_SNAP_CHANNEL}" = "latest/edge" ]; then
      lxc config get "user.microcloud" | grep -q "${microcloud_version}"
    fi

    lxc remote switch local

    # Remove the trust on the remote which was added when adding the remote.
    fingerprint="$(lxc query microcloud-test:/1.0/certificates?recursion=1 | jq -r '.[] | select(.name=="test") | .fingerprint')"
    lxc config trust remove "microcloud-test:${fingerprint}"

    # Remove the remote.
    lxc remote remove microcloud-test

    echo "==> ${name} Validated LXD"
}


# reset_snaps: Clears the state for existing snaps. This is a faster alternative to purging and re-installing snaps.
reset_snaps() {
  name="${1}"

  (
    set -eu
    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    # These are set to always pass in case the snaps are already disabled.
    echo "Disabling LXD and MicroCloud for ${name}"
    lxc exec "${name}" -- sh -c "
      rm -f /var/snap/lxd/common/lxd.debug
      rm -f /var/snap/microcloud/common/microcloudd.debug
      rm -f /var/snap/microcloud/common/microcloud.debug

      for app in lxd lxd.debug microcloud microcloud.debug microcloudd microcloudd.debug ; do
        if pidof -q \${app} > /dev/null; then
          kill -9 \$(pidof \${app})
        fi
      done

      snap disable lxd > /dev/null 2>&1 || true
      snap disable microcloud > /dev/null 2>&1 || true

      systemctl stop snap.lxd.daemon snap.lxd.daemon.unix.socket > /dev/null 2>&1 || true
      if ps -u lxd -o pid= ; then
        kill -9 \$(ps -u lxd -o pid=)
      fi

      rm -rf /var/snap/lxd/common/lxd
      rm -rf /var/snap/microcloud/*/*
    "

    echo "Resetting MicroCeph for ${name}"
    if lxc exec "${name}" -- snap list microceph > /dev/null 2>&1; then
      lxc exec "${name}" -- sh -c "
        snap disable microceph > /dev/null 2>&1 || true

        # Kill any remaining processes.
        # Filter out the subshell too to not kill our own invocation as it shows as 'sh -c ...microceph...' in the process list.
        if ps -e -o '%p %a' | grep -Ev '(grep|sh)' | grep -qe 'ceph-' -qe 'microceph' ; then
          kill -9 \$(ps -e -o '%p %a' | grep -Ev '(grep|sh)' | grep -e 'ceph-' -e 'microceph' | awk '{print \$1}') || true
        fi

        # Remove modules to get rid of any kernel owned processes.
        modprobe -r rbd ceph

        # Wipe the snap state so we can start fresh.
        rm -rf /var/snap/microceph/*/*
        snap enable microceph > /dev/null 2>&1 || true

        # microceph.osd requires this directory to exist but doesn't create it after install.
        # OSDs won't show up and ceph will freeze creating volumes without it, so make it here.
        mkdir -p /var/snap/microceph/current/run
        snap run --shell microceph -c 'snapctl restart microceph.osd' || true
      "
    fi

    echo "Resetting MicroOVN for ${name}"
    if lxc exec "${name}" -- snap list microovn > /dev/null 2>&1; then
      lxc exec "${name}" -- sh -c "
        microovn.ovn-appctl exit || true
        microovn.ovs-appctl exit --cleanup || true
        microovn.ovs-dpctl del-dp system@ovs-system || true
        snap disable microovn > /dev/null 2>&1 || true

        # Kill any remaining processes.
        # Filter out the subshell too to not kill our own invocation as it shows as 'sh -c ...microovn...' in the process list.
        if ps -e -o '%p %a' | grep -Ev '(grep|sh)' | grep -qe 'ovs-' -qe 'ovn-' -qe 'microovn' ; then
          kill -9 \$(ps -e -o '%p %a' | grep -Ev '(grep|sh)' | grep -e 'ovs-' -e 'ovn-' -e 'microovn' | awk '{print \$1}') || true
        fi

        # Wipe the snap state so we can start fresh.
        rm -rf /var/snap/microovn/*/*
        snap enable microovn > /dev/null 2>&1 || true
      "
    fi

    echo "Enabling LXD and MicroCloud for ${name}"
    lxc exec "${name}" -- snap enable lxd > /dev/null 2>&1 || true
    lxc exec "${name}" -- snap enable microcloud > /dev/null 2>&1 || true
    lxc exec "${name}" -- snap start lxd > /dev/null 2>&1 || true
    lxc exec "${name}" -- snap start microcloud > /dev/null 2>&1 || true
    lxc exec "${name}" -- lxd waitready

    set_debug_binaries "${name}"

    echo "Wait for all snaps in ${name} to settle"
    # No networks and storage pools are setup yet but the command will still wait for both MicroCloud and LXD to be ready.
    lxc exec "${name}" --env TEST_CONSOLE=0 -- microcloud waitready > /dev/null 2>&1 || true
  )
}

# reset_system: Starts the given system and resets its snaps, and devices.
# Makes only `num_disks` and `num_ifaces` disks and interfaces available for the next test.
reset_system() {
  if [ "${SNAPSHOT_RESTORE}" = 1 ]; then
    # shellcheck disable=SC2048,SC2086
    restore_system ${*}
    return
  fi

  name=$1
  num_disks=${2:-0}
  num_ifaces=${3:-0}

  echo "==> Resetting ${name} with ${num_disks} disk(s) and ${num_ifaces} extra interface(s)"
  (
    set -eu
    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null 2>&1
    fi

    lxc start "${name}" || true

    if [ -n "${MICROCLOUD_SNAP_PATH}" ]; then
      lxc file push --quiet "${MICROCLOUD_SNAP_PATH}" "${name}"/root/microcloud.snap
    fi

    lxc exec "${name}" -- ip link del lxdfan0 || true

    # Resync the time in case it got out of sync with the other VMs.
    lxc exec "${name}" --  timedatectl set-ntp off
    lxc exec "${name}" --  timedatectl set-ntp on

    # Rescan for any disks we hid from the previous run.
    lxc exec "${name}" -- sh -c "
      for h in /sys/class/scsi_host/host*; do
          echo '- - -' > \${h}/scan
      done
    "

    reset_snaps "${name}"

    # Attempt to destroy the zpool as we may have left dangling volumes when we wiped the LXD database earlier.
    # This is slightly faster than deleting the volumes by hand on each system.
    lxc exec "${name}" -- zpool destroy -f local || true

    # Hide any extra disks for this run.
    lxc exec "${name}" -- sh -c "
      disks=\$(lsblk -o NAME,SERIAL | grep \"lxd_disk[0-9]\" | cut -d\" \" -f1 | tac)
      count_disks=\$(echo \"\${disks}\" | wc -l)
      for d in \${disks} ; do
        if [ \${count_disks} -gt ${num_disks} ]; then
          echo \"Deleting /dev/\${d}\"
          echo 1 > /sys/block/\${d}/device/delete
        else
          echo \"Wiping /dev/\${d}\"
          wipefs --quiet -af /dev/\${d}
          dd if=/dev/zero of=/dev/\${d} bs=4096 count=100 status=none
        fi

        count_disks=\$((count_disks - 1))
      done
    "

    # Disable all extra interfaces.
    max_ifaces=$(lxc network ls -f csv | grep -cF microbr)
    for i in $(seq 1 "${max_ifaces}") ; do
      iface="enp$((i + 5))s0"
      lxc exec "${name}" -- ip link set "${iface}" down
    done

    # Re-enable as many interfaces as we want for this run.
    for i in $(seq 1 "${num_ifaces}") ; do
      iface="enp$((i + 5))s0"
      lxc exec "${name}" -- ip addr flush dev "${iface}"
      lxc exec "${name}" -- ip link set "${iface}" up
      lxc exec "${name}" -- sysctl -wq "net.ipv6.conf.${iface}.disable_ipv6=1"
    done
  )

  echo "==> Reset ${name}"
}

# cluster_reset: Resets cluster-wide settings in preparation for reseting test nodes.
cluster_reset() {
  name=${1}
  (
    set -eu
    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null 2>&1
    fi

    lxc exec "${name}" -- sh -c "
      for m in \$(lxc ls -f csv -c n) ; do
        lxc rm \$m -f
      done

      for f in \$(lxc image ls -f csv -c f) ; do
        lxc image rm \$f
      done

      echo 'config: {}' | lxc profile edit default || true
    "

    if lxc exec "${name}" -- snap list microceph > /dev/null 2>&1; then
      lxc exec "${name}" -- sh -c "
        # Ceph might not be responsive if we haven't set it up yet.
        microceph_setup=0
        if timeout -k 3 3 microceph cluster list ; then
          microceph_setup=1
        fi

        if [ \$microceph_setup = 1 ]; then
          microceph.ceph tell mon.\* injectargs '--mon-allow-pool-delete=true'
          lxc storage rm remote || true
          microceph.rados purge lxd_remote --yes-i-really-really-mean-it --force
          microceph.ceph fs fail lxd_cephfs || true
          microceph.ceph fs rm lxd_cephfs  --yes-i-really-mean-it || true
          microceph.rados purge lxd_cephfs_meta --yes-i-really-really-mean-it --force || true
          microceph.rados purge lxd_cephfs_data --yes-i-really-really-mean-it --force || true
          microceph.rados purge .mgr --yes-i-really-really-mean-it --force

          for pool in \$(microceph.ceph osd pool ls) ; do
            microceph.ceph osd pool rm \${pool} \${pool} --yes-i-really-really-mean-it
          done

          microceph.ceph osd set noup
          for osd in \$(microceph.ceph osd ls) ; do
            microceph.ceph config set osd.\${osd} osd_pool_default_crush_rule \$(microceph.ceph osd crush rule dump microceph_auto_osd | jq '.rule_id')
            microceph.ceph osd crush reweight osd.\${osd} 0
            microceph.ceph osd out \${osd}
            microceph.ceph osd down \${osd} --definitely-dead
            pkill -f \"ceph-osd .* --id \${osd}\"
            microceph.ceph osd purge \${osd} --yes-i-really-mean-it --force
            microceph.ceph osd destroy \${osd} --yes-i-really-mean-it --force
            rm -rf /var/snap/microceph/common/data/osd/ceph-\${osd}
          done
        fi
      "
    fi
  )
}

# reset_systems: Concurrently or sequentially resets the specified number of systems.
reset_systems() {
  collect_go_cover_files

  if [ "${SNAPSHOT_RESTORE}" = 1 ]; then
    # shellcheck disable=SC2048,SC2086
    restore_systems ${*}
    return
  fi

  echo "::group::reset_systems"

  num_vms=3
  num_disks=3
  num_ifaces=1

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_vms="${1}"
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_disks="${1}"
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_ifaces="${1}"
    shift 1
  fi

  for i in $(seq -f "%02g" 1 "${num_vms}") ; do
    name="micro${i}"
    if [ "${name}" = "micro01" ]; then
      cluster_reset "${name}"
    fi

    if [ "${CONCURRENT_SETUP}" = 1 ]; then
      reset_system "${name}" "${num_disks}" "${num_ifaces}" &
    else
      reset_system "${name}" "${num_disks}" "${num_ifaces}"
    fi
  done

  # Pause any extra systems.
  total_machines="$(lxc list -f csv -c n micro | wc -l)"
  for i in $(seq -f "%02g" "$((1 + num_vms))" "${total_machines}"); do
    name="micro${i}"
    lxc pause "${name}" || true
  done

  wait

  echo "::endgroup::"
}

# restore_systems: Restores the systems from a snapshot at snap0.
restore_systems() {
  echo "::group::restore_systems"

  collect_go_cover_files

  num_vms=3
  num_disks=3
  num_extra_ifaces=1

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_vms=${1}
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_disks=${1}
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_extra_ifaces=${1}
    shift 1
  fi

  lxc stop --all --force

  (
    set -eu
    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    for i in $(seq 1 "${num_extra_ifaces}") ; do
      network="microbr$((i - 1))"
      lxc profile device remove default "eth${i}"
      lxc network delete "${network}" || true
      lxc network create "${network}" \
        ipv4.address="10.${i}.123.1/24" ipv4.dhcp=false ipv4.nat=true \
        ipv6.address="fd42:${i}:1234:1234::1/64" ipv6.nat=true

      lxc profile device add default "eth${i}" nic network="${network}" name="eth${i}"
    done
  )

  for n in $(seq -f "%02g" 1 "${num_vms}") ; do
    name="micro${n}"
    if [ "${CONCURRENT_SETUP}" = 1 ]; then
      restore_system "${name}" "${num_disks}" "${num_extra_ifaces}" &
    else
      restore_system "${name}" "${num_disks}" "${num_extra_ifaces}"
    fi
  done

  wait

  echo "::endgroup::"
}

restore_system() {
  name="${1}"
  shift 1

  num_disks="0"
  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_disks="${1}"
    shift 1
  fi

  num_extra_ifaces="0"
  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_extra_ifaces="${1}"
    shift 1
  fi

  echo "==> Restoring ${name} from snapshot snap0 with ${num_disks} fresh disks and ${num_extra_ifaces} extra interfaces"

  (
    set -eu

    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    lxc remote switch local
    lxc project switch microcloud-test

    if [ "$(lxc list "${name}" -f csv -c s)" = "RUNNING" ]; then
      lxc stop "${name}" --force
    fi

    lxc restore "${name}" snap0

    for disk in $(lxc config device list "${name}") ; do
      if lxc config device get "${name}" "${disk}" type | grep -qF "disk" ; then
        lxc config device remove "${name}" "${disk}"
      fi

      volume="${name}-${disk}"
      if lxc storage volume list zpool -f csv | grep -q "^custom,${volume}" ; then
        lxc storage volume delete zpool "${volume}"
      fi
    done


    for n in $(seq 1 "${num_disks}") ; do
      disk="${name}-disk${n}"
      lxc storage volume create zpool "${disk}" size=10GiB --type=block
      lxc config device add "${name}" "disk${n}" disk pool=zpool source="${disk}"
    done

    lxc start "${name}"

    lxd_wait_vm "${name}"

    # Sleep some time so the snaps are fully set up.
    sleep 3

    for i in $(seq 1 "${num_extra_ifaces}") ; do
      network="enp$((i + 5))s0"
      lxc exec "${name}" -- ip link set "${network}" up
      lxc exec "${name}" -- sysctl -wq "net.ipv6.conf.${network}.disable_ipv6=1"
    done
  )

  set_debug_binaries "${name}"
  echo "==> Restored ${name}"
}


# cleanup: try to clean everything that is in the lxd-cloud project
cleanup_systems() {
  lxc remote switch local
  if lxc remote list -f csv | cut -d',' -f1 | grep -qF microcloud-test; then
      lxc remote remove microcloud-test || true
  fi
  lxc project switch microcloud-test
  echo "==> Removing systems"
  lxc list -c n -f csv | xargs --no-run-if-empty lxc delete --force

  for profile in $(lxc profile list -f csv | cut -d, -f1 | grep -vxF default); do
    lxc profile delete "${profile}"
  done

  for volume in $(lxc storage volume list -f csv -c t,n zpool | grep -F "custom," | cut -d',' -f2-); do
    lxc storage volume delete zpool "${volume}"
  done

  echo 'config: {}' | lxc profile edit default

  lxc remote switch local
  lxc project switch default
  lxc project delete microcloud-test

  for net in $(lxc network ls -f csv | grep microbr | cut -d',' -f1) ; do
    lxc network delete "${net}"
  done

  lxc storage delete zpool
  echo "==> All systems removed"
}

# setup_lxd: create a dedicate project in the host's LXD to use for the testbed
#            it also sets core.https_address to make the LXD API available to MAAS and Juju
setup_lxd_project() {
  # Create project
  (
    set -eu

    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    lxc remote switch local
    lxc project create microcloud-test -c features.images=false || true
    lxc project switch microcloud-test

    # Create a zfs pool so we can use fast snapshots.
    if [ -z "${TEST_STORAGE_SOURCE:-}" ]; then
      lxc storage create zpool zfs volume.size=5GiB
    else
      sudo wipefs --all --quiet "${TEST_STORAGE_SOURCE}"
      sudo blkdiscard "${TEST_STORAGE_SOURCE}" || true
      lxc storage create zpool zfs source="${TEST_STORAGE_SOURCE}"
    fi

    # Setup default profile
    cat << EOF | lxc profile edit default
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

    lxc profile set default boot.autostart true
    lxc profile device add default root disk pool=zpool path=/
    lxc profile device add default eth0 nic network=lxdbr0 name=eth0

    lxc profile set default environment.TEST_CONSOLE=1
    lxc profile set default environment.DEBIAN_FRONTEND=noninteractive
    lxc profile set default environment.PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"
  )
}

create_system() {
  name="${1}"
  num_disks="${2:-0}"
  shift 2

  os="${BASE_OS:-24.04}"

  echo "==> ${name} Creating VM with ${num_disks} disks"
  (
    set -eu

    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    # Pre fetch additional images to be used by the VM through security.devlxd.images=true
    lxc image copy ubuntu-minimal-daily:24.04 local:
    lxc image copy ubuntu-minimal-daily:22.04 local:
    if [ "${SKIP_VM_LAUNCH}" != "1" ]; then
        lxc image copy ubuntu-minimal-daily:24.04 local: --vm
        lxc image copy ubuntu-minimal-daily:22.04 local: --vm
    fi

    lxc init "ubuntu-minimal-daily:${os}" "${name}" --vm -c limits.cpu=4 -c limits.memory=4GiB -c security.devlxd.images=true

    # Disable vGPU to save RAM
    lxc config set "${name}" raw.qemu.conf='[device "qemu_gpu"]'

    for n in $(seq 1 "${num_disks}") ; do
      disk="${name}-disk${n}"
      lxc storage volume create zpool "${disk}" size=10GiB --type=block
      lxc config device add "${name}" "disk${n}" disk pool=zpool source="${disk}"
    done

    lxc start "${name}"
  )
}

retry() {
    local cmd="$*"
    local maxAttempts="10"
    local i

    for i in $(seq 1 "${maxAttempts}"); do
        # Make sure to catch both stdout and stderr.
        # Also kill the command after 30 minutes if it is still running.
        # shellcheck disable=SC2086
        output="$(timeout 30m ${cmd} 2>&1)" && return 0

        # Log the failed attempt so that it's visible on the pipeline's summary page.
        echo "::warning::Failed to run '${cmd}': ${output}"

        retryInSeconds="$((i*5))"
        echo "Retrying in ${retryInSeconds} seconds... (${i}/${maxAttempts})"
        sleep "${retryInSeconds}"
    done

    return 1
}

setup_system() {
  name="${1}"
  shift 1

  echo "==> ${name} Setting up"

  (
    set -eu

    if [ "${SKIP_SETUP_LOG}" = 1 ]; then
      exec > /dev/null
    fi

    # Disable unneeded services/timers/sockets/mounts (source of noise/slowdown)
    lxc exec "${name}" -- systemctl mask --now apport.service cron.service e2scrub_reap.service esm-cache.service grub-common.service grub-initrd-fallback.service networkd-dispatcher.service polkit.service secureboot-db.service serial-getty@ttyS0.service ssh.service systemd-journal-flush.service unattended-upgrades.service
    lxc exec "${name}" -- systemctl mask --now apt-daily-upgrade.timer apt-daily.timer dpkg-db-backup.timer e2scrub_all.timer fstrim.timer motd-news.timer update-notifier-download.timer update-notifier-motd.timer
    lxc exec "${name}" -- systemctl mask --now iscsid.socket
    lxc exec "${name}" -- systemctl mask --now dev-hugepages.mount sys-kernel-debug.mount sys-kernel-tracing.mount

    # Turn off debugfs and mitigations
    echo 'GRUB_CMDLINE_LINUX_DEFAULT="quiet debugfs=off mitigations=off"' | lxc exec "${name}" -- tee /etc/default/grub.d/zz-lxd-speed.cfg
    lxc exec "${name}" -- update-grub

    # Faster apt
    echo "force-unsafe-io" | lxc exec "${name}" -- tee /etc/dpkg/dpkg.cfg.d/force-unsafe-io

    # Remove unneeded/unwanted packages
    lxc exec "${name}" -- apt-get autopurge -y lxd-installer

    packages="jq"

    # Install the snaps.
    # Retry the attempts in case the download from external sources fails due to instability.
    retry lxc exec "${name}" -- apt-get update
    if [ -n "${CLOUD_INSPECT:-}" ] || [ "${SNAPSHOT_RESTORE}" = 0 ]; then
      packages+=" zfsutils-linux htop"
    fi

    if [ ! "${BASE_OS}" = "22.04" ]; then
      packages+=" yq"
    else
      retry lxc exec "${name}" -- snap install yq
    fi

    # shellcheck disable=SC2086
    retry lxc exec "${name}" -- apt-get install --no-install-recommends -y ${packages}
    retry lxc exec "${name}" -- snap install snapd

    # Free disk blocks
    lxc exec "${name}" -- apt-get clean
    lxc exec "${name}" -- systemctl start fstrim.service

    # Snaps can occasionally fail to install properly, so repeatedly try.
    lxc exec "${name}" -- sh -c "
      while ! test -e /snap/bin/microceph ; do
        snap install microceph --channel=\"${MICROCEPH_SNAP_CHANNEL}\" --cohort='+' || true
        sleep 1
      done

      if [ ! \"${BASE_OS}\" = \"22.04\" ]; then
        # dm-crypt needs to be manually connected for microceph full disk encyption.
        snap connect microceph:dm-crypt
        snap restart microceph.daemon
      fi

      while ! test -e /snap/bin/microovn ; do
        snap install microovn --channel=\"${MICROOVN_SNAP_CHANNEL}\" --cohort='+' || true
        sleep 1
      done

      if test -e /snap/bin/lxd ; then
        snap remove lxd --purge
      fi

      while ! test -e /snap/bin/lxd ; do
        snap install lxd --channel=\"${LXD_SNAP_CHANNEL}\" --cohort='+' || true
        sleep 1
      done
    "

    # Silence the "If this is your first time running LXD on this machine" banner
    # on first invocation
    lxc exec "${name}" -- mkdir -p /root/snap/lxd/common/config/
    lxc exec "${name}" -- touch /root/snap/lxd/common/config/config.yml

    if [ -n "${MICROCLOUD_SNAP_PATH}" ]; then
      lxc file push --quiet "${MICROCLOUD_SNAP_PATH}" "${name}"/root/microcloud.snap
      lxc exec "${name}" -- snap install --dangerous /root/microcloud.snap
    else
      retry lxc exec "${name}" -- snap install microcloud --channel="${MICROCLOUD_SNAP_CHANNEL}" --cohort="+"
    fi

    # Hold the snaps to not perform any refreshes during test execution.
    # This can cause various side effects in case of restoring an old test deployment
    # as the snaps will identify upstream changes and initiate a refresh.
    # It was also observed in the test pipeline jobs from time to time.
    lxc exec "${name}" -- snap refresh --hold microceph microovn lxd microcloud

    set_debug_binaries "${name}"
  )

  # let boot/cloud-init finish its job
  waitInstanceBooted "${name}" || lxc exec "${name}" -- systemctl --failed || true

  # Create a snapshot so we can restore to this point.
  if [ "${SNAPSHOT_RESTORE}" = 1 ]; then
    lxc stop "${name}"
    lxc snapshot "${name}" snap0
  else
    # Sleep some time so the snaps are fully set up.
    sleep 3
  fi

  echo "==> ${name} Finished Setting up"
}

# Creates a new system with the given number of disks.
new_system() {
  name=${1}
  num_disks=${2:-0}

  (
    set -eu
    # Sometimes, the cloud-init user script fails to run in a CI environment,
    # so we retry a few times.
    for i in $(seq 5); do
      create_system "${name}" "${num_disks}"

      if ! lxd_wait_vm "${name}"; then
        echo "lxd_wait_vm failed, removing ${name} and retrying (attempt ${i})"
        lxc delete "${name}" -f
        for n in $(seq 1 "${num_disks}") ; do
          disk="${name}-disk${n}"
          lxc storage volume delete zpool "${disk}"
        done
      else
        break
      fi

      if [ "${i}" = 5 ]; then
        echo "Failed to create ${name} after 5 attempts"
        exit 1
      fi
    done
  )

  # Sleep some time so the vm is fully set up.
  sleep 3
  setup_system "${name}"
}

new_systems() {
  echo "::group::new_systems"

  num_vms=3
  num_disks=3
  num_ifaces=1

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_vms="${1}"
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_disks="${1}"
    shift 1
  fi

  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    num_ifaces="${1}"
    shift 1
  fi

  setup_lxd_project

  echo "==> Creating ${num_ifaces} extra network interfaces"
  for i in $(seq 1 "${num_ifaces}"); do
    # Create uplink network
    lxc network create "microbr$((i - 1))" \
        ipv4.address="10.${i}.123.1/24" ipv4.dhcp=false ipv4.nat=true \
        ipv6.address="fd42:${i}:1234:1234::1/64" ipv6.nat=true
    lxc profile device add default "eth${i}" nic network="microbr$((i - 1))" name="eth${i}"
  done

  for n in $(seq -f "%02g" 1 "${num_vms}"); do
    name="micro${n}"
    if [ "${CONCURRENT_SETUP}" = 1 ]; then
      new_system "${name}" "${num_disks}" &
    else
      new_system "${name}" "${num_disks}"
    fi
  done

  wait

  echo "::endgroup::"
}

wait_snapd() {
  name="${1}"

  if timeout 60s lxc exec "${name}" -- snap wait system seed.loaded; then
    return 0 # Success.
  fi

  echo "snapd not seeded after ${i}s"
  return 1 # Failed.
}

lxd_wait_vm() {
  name="${1}"

  echo "==> ${name} Awaiting VM..."
  sleep 5
  for round in $(seq 1 5 150); do
    if [ "$(lxc list -f csv -c s "${name}")" = "READY" ] ; then
      wait_snapd "${name}"
      echo "    ${name} VM is ready"
      return 0
    fi

    # Sometimes the VM just won't start, so retry after 3 minutes.
    if [ "$((round % 60))" = 0 ]; then
      echo "==> ${name} Timeout (${round}s): Re-initializing VM"
      lxc restart "${name}" --force
    fi

    sleep 5
  done

  echo "    ${name} VM failed to start"
  return 1
}

# ip_config_to_netaddr: Returns the IPv4 network address of the given interface.
# e.g: ip_config_to_netaddr lxdbr0 (with inet: 10.233.6.X/24)-> 10.233.6.0/24
ip_config_to_netaddr () {
  local ip ip_dec cidr mask net net_dec
  IFS=/ read -r ip cidr < <(ip -4 addr show dev "${1}" | awk '{if ($1 == "inet") print $2}')

  # Convert IP to a 32-bit integer (decimal)
  ip_dec="$(awk -F'.' '{printf "%d", (($1*256+$2)*256+$3)*256+$4}' <<< "${ip}")"

  # Calculate the network mask (32-bit integer)
  # Network Mask = (2^mask_len - 1) shifted left by (32 - mask_len)
  # A simpler way: (0xFFFFFFFF << (32 - mask_len)) & 0xFFFFFFFF
  mask="$(( (0xFFFFFFFF << (32 - cidr)) & 0xFFFFFFFF ))"

  # Calculate the network address (bit-wise AND)
  net_dec="$(( ip_dec & mask ))"

  # Convert the resulting 32-bit integer back to dotted-decimal format (network address)
  net="$(( (net_dec >> 24) & 0xFF )).$(( (net_dec >> 16) & 0xFF )).$(( (net_dec >> 8) & 0xFF )).$(( net_dec & 0xFF ))"

  # Output the final network address in CIDR format
  echo "${net}/${cidr}"
}

set_cluster_subnet() {
  num_systems="${1}"
  iface="${2}"
  prefix="${3}"

  shift 3

  for n in $(seq 2 $((num_systems + 1))); do
    cluster_ip="${prefix}.${n}/24"
    name="$(printf "micro%02d" $((n-1)))"
    lxc exec "${name}" -- ip addr flush "${iface}"
    lxc exec "${name}" -- ip addr add "${cluster_ip}" dev "${iface}"
  done
}

# waitInstanceReady: waits for the instance to be ready (processes count > 1).
waitInstanceReady() (
  { set +x; } 2>/dev/null
  maxWait="${MAX_WAIT_SECONDS:-120}"
  instName="${1}"

  # Wait for the instance to report more than one process.
  processes=0
  for _ in $(seq "${maxWait}"); do
      processes="$(lxc info "${instName}" | awk '{if ($1 == "Processes:") print $2}')"
      if [ "${processes:-0}" -ge "${MIN_PROC_COUNT:-2}" ]; then
          return 0 # Success.
      fi
      sleep 1
  done

  echo "Instance ${instName} not ready after ${maxWait}s"
  return 1 # Failed.
)

# waitInstanceBooted: waits for the instance to be ready and fully booted.
waitInstanceBooted() (
  { set +x; } 2>/dev/null
  prefix="${WARNING_PREFIX:-::warning::}"
  maxWait=90
  instName="$1"

  # Wait for the instance to be ready
  waitInstanceReady "${instName}"

  # Then wait for the boot sequence to complete.
  sleep 1
  rc=0
  state="$(lxc exec "${instName}" -- timeout "${maxWait}" systemctl is-system-running --wait)" || rc="$?"

  # rc=124 is when `timeout` is hit.
  # Other rc values are ignored as it doesn't matter if the system is fully
  # operational (`running`) as it is booted.
  if [ "${rc}" -eq 124 ]; then
    echo "${prefix}Instance ${instName} not booted after ${maxWait}s"
    lxc list "${instName}"
    return 1 # Failed.
  elif [ "${state}" != "running" ]; then
    echo "${prefix}Instance ${instName} booted but not fully operational: ${state} != running"
  fi

  return 0 # Success.
)
