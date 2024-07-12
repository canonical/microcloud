#!/bin/bash

test_recover() {
  reset_systems 4 0 0

  systems=("micro01" "micro02" "micro03" "micro04")

  lxc exec micro01 -- sh -c "TEST_CONSOLE=0 microcloud init --auto > out"
  for m in "${systems[@]}" ; do
    validate_system_lxd "${m}" 4
    validate_system_microceph "${m}"
    validate_system_microovn "${m}"
  done

  # MicroCluster takes a while to update the core_cluster_members table
  while lxc exec micro01 -- microcloud cluster list -f csv | grep -q PENDING; do
    sleep 2
  done

  for m in "${systems[@]}"; do
    lxc exec "${m}" -- sudo snap stop microcloud
  done

  lxc exec micro01 -- microcloud cluster list --local -f yaml

  lxc exec micro01 -- sh -c "
    TEST_CONSOLE=0 microcloud cluster list --local -f yaml |
      yq '
        sort_by(.name) |
        .[0].role = \"voter\" |
        .[1].role = \"voter\" |
        .[2].role = \"spare\" |
        .[3].role = \"spare\"' |
      TEST_CONSOLE=0 microcloud cluster recover"

  lxc file pull micro01/var/snap/microcloud/common/state/recovery_db.tar.gz ./
  lxc file push recovery_db.tar.gz micro02/var/snap/microcloud/common/state/recovery_db.tar.gz

  for m in micro01 micro02; do
    lxc exec "${m}" -- sudo snap start microcloud
  done

  # microcluster takes a long time to update the member roles in the core_cluster_members table
  sleep 90

  for m in micro01 micro02; do
    cluster_list=$(lxc exec "${m}" -- microcloud cluster list -f csv)

    # assert_member_role(member_name, role)
    assert_member_role() {
      [[ $(echo "${cluster_list}" | grep "${1}" | awk -F, '{print $3}') == "${2}" ]]
    }

    assert_member_role micro01 voter
    assert_member_role micro02 voter

    for spare_member in micro03 micro04; do
      assert_member_role "${spare_member}" spare
    done
  done
}
