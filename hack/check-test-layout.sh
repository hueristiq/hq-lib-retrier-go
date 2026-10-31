#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

report() {
	printf '%s\n' "$*" >>"$tmp"
}

# CHECK_TEST_LAYOUT_EXCLUDE is a space-separated list of directory paths
# (relative to the repository root) to skip entirely. Generated and vendored
# trees keep their own conventions.
is_excluded_dir() {
	case "$1" in
	'')
		return 1
		;;
	esac

	for pattern in ${CHECK_TEST_LAYOUT_EXCLUDE:-}; do
		case "$1" in
		"$pattern" | "$pattern"/*) return 0 ;;
		esac
	done

	return 1
}

is_exempt_source() {
	name=$(basename -- "$1")

	case "$name" in
	doc.go | z*.go | *_gen.go | *_generate.go | *_other.go | *_plan9.go | *_js.go | *_wasip1.go)
		return 0
		;;
	esac

	return 1
}

for banned in pkg utils helpers models common; do
	find . -type d -name "$banned" \
		-not -path './.git/*' -not -path './.bin/*' -not -path './.github/*' \
		-not -path './.vscode/*' -not -path './docs/*' -not -path './testdata/*' \
		| while read -r dir; do
		printf 'discouraged directory name: %s\n' "$dir" >>"$tmp"
	done
done

find . -name '*.go' -not -name '*_test.go' \
	-not -path './.git/*' -not -path './.bin/*' -not -path './.github/*' \
	-not -path './.vscode/*' -not -path './docs/*' -not -path './examples/*' \
	-not -path './cmd/*' -not -path './testdata/*' \
	-exec dirname {} \; | sort -u | while read -r dir; do
	if is_excluded_dir "$dir"; then
		continue
	fi

	pkg=$(awk '/^package /{print $2; exit}' "$dir"/*.go)
	external=0

	for test in "$dir"/*_test.go; do
		[ -e "$test" ] || continue

		test_pkg=$(awk '/^package /{print $2; exit}' "$test")

		case "$test_pkg" in
		*_test)
			external=1
			;;
		*)
			case $(basename -- "$test") in
			*_internal_test.go | *_bench_test.go | *_fuzz_test.go) ;;
			*) printf 'same-package test must end in _internal_test.go: %s\n' "$test" >>"$tmp" ;;
			esac
			;;
		esac
	done

	case "$dir" in
	./internal | ./internal/* | */internal | */internal/*) ;;
	*)
		if [ "$external" -eq 0 ]; then
			printf 'missing external test for public package %s: %s\n' "$pkg" "$dir" >>"$tmp"
		fi
		;;
	esac

	for src in "$dir"/*.go; do
		[ -e "$src" ] || continue

		base=$(basename -- "$src" .go)

		case "$base" in
		*_test) continue ;;
		esac

		if is_exempt_source "$src"; then
			continue
		fi

		if [ ! -e "$dir/${base}_test.go" ] && [ ! -e "$dir/${base}_internal_test.go" ]; then
			printf 'missing test for implementation file: %s\n' "$src" >>"$tmp"
		fi
	done
done

if [ -s "$tmp" ]; then
	cat "$tmp" >&2
	exit 1
fi
