# Miscellaneous test checks.

check_dependencies() {
	# shellcheck disable=SC3043
	local dep missing
	missing=""

	for dep in "$@"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			[ "$missing" ] && missing="$missing $dep" || missing="$dep"
		fi
	done

	if [ "$missing" ]; then
		echo "Missing dependencies: $missing" >&2
		exit 1
	fi

	# Instances need to be able to self-report on their state
	if ! lxc info | sed -ne '/^api_extensions:/,/^[^-]/ s/^- //p' | grep -qxF "instance_ready_state"; then
		echo "Missing LXD instance_ready_state extension" >&2
		exit 1
	fi
}

check_empty() {
	if [ "$(find "${1}" 2>/dev/null | wc -l)" -gt "1" ]; then
		echo "${1} is not empty, content:"
		find "${1}"
		false
	fi
}

check_snap_channels() {
    non_edge=""
    if [ "${LXD_SNAP_CHANNEL}" != "latest/edge" ]; then
        non_edge="${non_edge} lxd"
    fi
    if [ "${MICROCEPH_SNAP_CHANNEL}" != "latest/edge" ]; then
        non_edge="${non_edge} microceph"
    fi
    if [ "${MICROCLOUD_SNAP_CHANNEL}" != "latest/edge" ]; then
        non_edge="${non_edge} microcloud"
    fi
    if [ "${MICROOVN_SNAP_CHANNEL}" != "latest/edge" ]; then
        non_edge="${non_edge} microovn"
    fi

    for snap in ${non_edge}; do
        echo "::warning::${snap} channel not set to latest/edge, continuing anyway"
    done
}
