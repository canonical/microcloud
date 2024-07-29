#!/bin/sh -eu

echo "Checking for error messages beginning with lower-case letters..."

! git grep --untracked -P -n 'fmt\.Errorf\("[a-z]' -- '*.go'
