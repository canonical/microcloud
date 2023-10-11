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
}

check_empty() {
	if [ "$(find "${1}" 2>/dev/null | wc -l)" -gt "1" ]; then
		echo "${1} is not empty, content:"
		find "${1}"
		false
	fi
}
