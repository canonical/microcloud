#!/bin/bash
set -eu
[ -n "${GOPATH:-}" ] && export "PATH=${GOPATH}/bin:${PATH}"

# Don't translate lxc output for parsing in it in tests.
export LC_ALL="C"

# Force UTC for consistency
export TZ="UTC"

# Tell debconf to not be interactive
export DEBIAN_FRONTEND=noninteractive

export LOG_LEVEL_FLAG=""
if [ -n "${VERBOSE:-}" ]; then
	LOG_LEVEL_FLAG="--verbose"
fi

if [ -n "${DEBUG:-}" ]; then
	LOG_LEVEL_FLAG="--debug"
	set -x
fi

import_subdir_files() {
	test "$1"
	# shellcheck disable=SC3043
	local file
	for file in "$1"/*.sh; do
		# shellcheck disable=SC1090
		. "$file"
	done
}

import_subdir_files includes

echo "==> Checking for dependencies"
check_dependencies lxc lxd curl awk jq git python3 shuf rsync openssl

cleanup() {
	# Do not exit if commands fail on cleanup. (No need to reset -e as this is only run on test suite exit).
	set -eux
	lxc remote switch local
	lxc project switch microcloud-test
	set +e

	if [ "${TEST_CURRENT}" = "setup" ] && [ "${TEST_RESULT}" = "success" ]; then
		return
	fi

	# Allow for inspection
	if [ -n "${CLOUD_INSPECT:-}" ]; then
		if [ "${TEST_RESULT}" != "success" ]; then
			echo "==> TEST DONE: ${TEST_CURRENT_DESCRIPTION}"
		fi
		echo "==> Test result: ${TEST_RESULT}"

		echo "Tests Completed (${TEST_RESULT}): hit enter to continue"
		read -r _
	fi

	echo "::group::debug-failure"
	lxc list --all-projects || true
	lxc exec micro01 -- lxc list || true

	for name in $(lxc list -c n -f csv micro); do
		echo "Check LXD resources on ${name} for disk ordering"
		lxc exec "${name}" -- lxc query "/1.0/resources" | jq -r '.storage.disks[] | {id, device_id, device_path}'
		lxc exec "${name}" -- lsblk
	done

	for name in $(lxc list -c n -f csv micro); do
		echo -n "${name} out file:"
        if ! lxc exec "${name}" -- test -e out; then
            echo " was not found"
            continue
        elif ! lxc exec "${name}" -- test -s out; then
            echo " was empty"
            continue
        fi
        echo
		lxc exec "${name}" -- cat out
	done
	echo "::endgroup::"

	# LXD daemon logs
	echo "::group::lxd logs"
	journalctl --quiet --no-hostname --no-pager --boot=0 --lines=100 --unit=snap.lxd.daemon.service
	echo "::endgroup::"

	# dmesg may contain oops, IO errors, crashes, etc
	echo "::group::dmesg logs"
	journalctl --quiet --no-hostname --no-pager --boot=0 --lines=100 --dmesg
	echo "::endgroup::"

	if [ -n "${GITHUB_ACTIONS:-}" ]; then
		echo "==> Skipping cleanup (GitHub Action runner detected)"
	else
		echo "==> Cleaning up"
		cleanup_systems
	fi

	echo ""
	echo ""
	if [ "${TEST_RESULT}" != "success" ]; then
		echo "==> TEST DONE: ${TEST_CURRENT_DESCRIPTION}"
	fi
	echo "==> Test result: ${TEST_RESULT}"

    if [ "${CONCURRENT_SETUP}" = 1 ]; then
        # kill our whole process group
        kill -- -$$
    fi
}

# Must be set before cleanup()
TEST_CURRENT=setup
TEST_CURRENT_DESCRIPTION=setup
# shellcheck disable=SC2034
TEST_RESULT=failure

trap cleanup EXIT HUP INT TERM

# Import all the testsuites
import_subdir_files suites

LXD_SNAP_CHANNEL="${LXD_SNAP_CHANNEL:-latest/edge}"
export LXD_SNAP_CHANNEL

MICROCEPH_SNAP_CHANNEL="${MICROCEPH_SNAP_CHANNEL:-latest/edge}"
export MICROCEPH_SNAP_CHANNEL

MICROCLOUD_SNAP_CHANNEL="${MICROCLOUD_SNAP_CHANNEL:-latest/edge}"
export MICROCLOUD_SNAP_CHANNEL

MICROOVN_SNAP_CHANNEL="${MICROOVN_SNAP_CHANNEL:-latest/edge}"
export MICROOVN_SNAP_CHANNEL

MICROOVN_SNAP_PATH="${MICROOVN_SNAP_PATH:-}"
export MICROOVN_SNAP_PATH

MICROCEPH_SNAP_PATH="${MICROCEPH_SNAP_PATH:-}"
export MICROCEPH_SNAP_PATH

CONCURRENT_SETUP=${CONCURRENT_SETUP:-0}
export CONCURRENT_SETUP

SKIP_SETUP_LOG=${SKIP_SETUP_LOG:-0}
export SKIP_SETUP_LOG

SKIP_VM_LAUNCH=${SKIP_VM_LAUNCH:-0}
export SKIP_VM_LAUNCH

SNAPSHOT_RESTORE=${SNAPSHOT_RESTORE:-0}
export SNAPSHOT_RESTORE

TESTBED_READY=${TESTBED_READY:-0}
export TESTBED_READY

set +u
if [ -z "${MICROCLOUD_SNAP_PATH}" ] || ! [ -e "${MICROCLOUD_SNAP_PATH}" ]; then
  MICROCLOUD_SNAP_PATH=""
fi

if [ -z "${MICROCLOUD_DEBUG_PATH}" ] || ! [ -e "${MICROCLOUD_DEBUG_PATH}" ]; then
  MICROCLOUD_DEBUG_PATH=""
fi

if [ -z "${MICROCLOUDD_DEBUG_PATH}" ] || ! [ -e "${MICROCLOUDD_DEBUG_PATH}" ]; then
  MICROCLOUDD_DEBUG_PATH=""
fi

if [ -z "${LXD_DEBUG_PATH}" ] || ! [ -e "${LXD_DEBUG_PATH}" ]; then
  LXD_DEBUG_PATH=""
fi
set -u

export MICROCLOUD_SNAP_PATH

echo "===> Checking that all snap channels are set to latest/edge"
check_snap_channels

run_test() {
    if [ "${TESTBED_READY}" = 0 ]; then
        testbed_setup
    fi

	TEST_CURRENT="${1}"
	TEST_CURRENT_DESCRIPTION="${2:-${1}}"

	echo "==> TEST BEGIN: ${TEST_CURRENT_DESCRIPTION}"
	START_TIME="$(date +%s)"
	${TEST_CURRENT}
	END_TIME="$(date +%s)"

	echo "::notice::==> TEST DONE: ${TEST_CURRENT_DESCRIPTION} ($((END_TIME - START_TIME))s)"
}

# Create 4 nodes with 3 disks and 3 extra interfaces.
# These nodes should be used across most tests and reset with the `reset_systems` function.
testbed_setup() {
  echo "==> SETUP STARTED"
  START_TIME="$(date +%s)"

  new_systems 4 3 3
  TESTBED_READY=1

  END_TIME="$(date +%s)"
  echo "::notice::==> SETUP DONE ($((END_TIME - START_TIME))s)"
}

# test groups
run_add_tests() {
  run_test test_add_interactive "add interactive"
  run_test test_add_auto "add auto"
}

run_instances_tests() {
  run_test test_instances_config "instances config"
  run_test test_instances_launch "instances launch"
}

run_basic_tests() {
  run_test test_reuse_cluster "reuse_cluster"
  run_test test_add_services "add_services"
  run_test test_auto "auto"
  run_test test_remove_cluster_member "remove_cluster_member"
  run_test test_non_ha "non_ha"
}

run_recover_tests() {
  run_test test_recover "recover"
}

run_interactive_tests() {
  run_test test_interactive "interactive"
  run_test test_interactive_combinations "interactive combinations"
}

run_mismatch_tests() {
  run_test test_service_mismatch "service mismatch"
  run_test test_disk_mismatch "disk mismatch"
}

run_preseed_tests() {
  run_test test_preseed "preseed"
}

# allow for running a specific set of tests
if [ "${1:-"all"}" = "all" ]; then
  run_add_tests
  run_instances_tests
  run_basic_tests
  run_recover_tests
  run_interactive_tests
  run_mismatch_tests
  run_preseed_tests
elif [ "${1}" = "add" ]; then
  run_add_tests
elif [ "${1}" = "instances" ]; then
  run_instances_tests
elif [ "${1}" = "basic" ]; then
  run_basic_tests
elif [ "${1}" = "recover" ]; then
  run_recover_tests
elif [ "${1}" = "interactive" ]; then
  run_interactive_tests
elif [ "${1}" = "mismatch" ]; then
  run_mismatch_tests
elif [ "${1}" = "preseed" ]; then
  run_preseed_tests
elif [ "${1}" = "setup" ]; then
  testbed_setup
else
  run_test "test_${1}"
fi

# shellcheck disable=SC2034
TEST_RESULT=success
