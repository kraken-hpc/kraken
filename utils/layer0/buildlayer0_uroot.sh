#!/bin/bash

###
# This will build a layer0 image.
#
# Notes:
# - It's not very resilient.
# - It expects you already have built kraken in $GOPATH/github.com/hpc/kraken/build
# - It will attempt to install u-root if it doesn't find it in $GOPATH/bin
# - You can overlay a base directory by providing a second argument.
#
# Kernel Modules:
#  - If /modules.txt exists and contains a line-by-line of full paths (from <base_dir>/) to .ko kernel modules.
#  - uinit will insmod those modules in order before doing anything else.
#  - uinit will not resolve module dependencies, so list them in dependency order.
#  - u-root's insmod doesn't support compressed modules, so uncompress them first.
###

usage() {
        echo "Usage: $0 [-o <out_file>] [-b <base_dir>] [-k <kraken_source_dir>] [-B <kraken_build_dir>] <arch>"
        echo "  <arch> should be the GOARCH we want to build (e.g. arm64, amd64...)"
        echo "  <out_file> is the file the image should be written to.  (default: initramfs.<date>.<img>.cpio.gz)"
        echo "  <base_dir> is an optional base directory containing file/directory structure (default: none)"
        echo "             that should be added to the image"
        echo "  <kraken_source_dir> is the location of the kraken source (default: GOPATH/src/github.com/hpc/kraken)"
        echo "  <kraken_build_dir> specifies an alternate path where to look for the generated kraken source"
        echo "             tree to be used by u-root (default: <kraken_source_dir>/build)"
}

# Exit with a failure message
fatal() {
    echo "$1" >&2
    exit 1
}

if ! opts=$(getopt o:b:k:B: "$@"); then
    usage
    exit
fi

# shellcheck disable=SC2086
set -- $opts
for i; do
    case "$i"
    in
        -o)
            echo "Output file is $2"
            OUTFILE="$2"
            shift; shift;;
        -b)
            echo "Using base dir $2"
            BASEDIR="$2"
            shift; shift;;
        -k)
            echo "Using kraken source dir $2"
            KRAKEN_SOURCEDIR=$2
            shift; shift;;
        -B)
            echo "Using kraken build dir $2"
            KRAKEN_BUILDDIR=$2
            shift; shift;;
        --)
            shift; break;;
    esac
done

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

ARCH=$1

if [ -z "${GOPATH+x}" ]; then
    echo "GOPATH isn't set, using $HOME/go"
    GOPATH=$HOME/go
fi

# Commands to build into u-root busybox
EXTRA_COMMANDS=()
EXTRA_COMMANDS+=( github.com/jlowellwofford/entropy/cmd/entropy )
EXTRA_COMMANDS+=( github.com/jlowellwofford/uinit/cmds/uinit )
EXTRA_COMMANDS+=( github.com/bensallen/modscan/cmd/modscan )

if [ -z "${KRAKEN_SOURCEDIR+x}" ]; then
    KRAKEN_SOURCEDIR="$GOPATH/src/github.com/hpc/kraken"
    echo "Using kraken source dir $KRAKEN_SOURCEDIR"
fi

if [ -z "${KRAKEN_BUILDDIR+x}" ]; then
    KRAKEN_BUILDDIR="$KRAKEN_SOURCEDIR/build"
    echo "Using kraken build dir $KRAKEN_BUILDDIR"
fi

# make a temporary directory for our base
TMPDIR=$(mktemp -d)
echo "Using tmpdir: $TMPDIR"

# Check that kraken build dir is not empty
contents=$(ls -A "$KRAKEN_BUILDDIR")
if [ -z "$contents" ]; then
    echo "$KRAKEN_BUILDDIR is empty; build it before running this"
    exit 1
fi
echo "Using generated kraken source tree at $KRAKEN_BUILDDIR"

# Check that gobusybox is installed, clone it if not
if [ ! -d "$GOPATH"/bin/makebb ]; then
    echo "You don't appear to have gobusybox installed, attempting to install it"
    GOPATH="$GOPATH" GO111MODULE=off go get github.com/u-root/gobusybox/src/cmd/makebb
fi

# Check that u-root is installed, clone it if not
if [ ! -x "$GOPATH"/bin/u-root ]; then
    echo "You don't appear to have u-root installed, attempting to install it"
    GOPATH="$GOPATH" GO111MODULE=off go get github.com/u-root/u-root
fi

# Make sure commands are available
for c in "${EXTRA_COMMANDS[@]}"; do
   if [ ! -d "$GOPATH/src/$c" ]; then
    echo "You don't appear to have $c, attempting to install it"
    GOPATH=$GOPATH GO111MODULE=off go get "$c"
   fi
done

# Resolve the u-root dependency conflict between local copy and that required by
# github.com/jlowellwofford/uinit according to:
# https://github.com/u-root/gobusybox#common-dependency-conflicts
go mod edit -replace=github.com/u-root/u-root=../../u-root/u-root "$GOPATH"/src/github.com/jlowellwofford/uinit/go.mod

# Delete vendor/ directory so that makebb will only see modules and not throw the error:
# "busybox does not support mixed module/non-module compilation"
# Also, modify the go.mod to use the local u-root.
# This is a hack.
(
 modscan_path="$GOPATH"/src/github.com/bensallen/modscan
 cd "${modscan_path}" || fatal "Could not enter ${modscan_path}. Does it exist?"
 go mod edit -replace=github.com/u-root/u-root=../../u-root/u-root go.mod
 rm -rf vendor/
)

# Generate the array of commands to add to BusyBox binary
BB_COMMANDS=( "$GOPATH"/src/github.com/u-root/u-root/cmds/{core,boot,exp}/* )
BB_COMMANDS+=( "$GOPATH"/src/github.com/hpc/kraken/build/u-root/kraken )
# shellcheck disable=SC2068
for cmd in ${EXTRA_COMMANDS[@]}; do
    BB_COMMANDS+=( "$GOPATH"/src/"$cmd" )
done

# Create BusyBox binary (outside of u-root)
echo "Creating BusyBox binary..."
mkdir -p "$TMPDIR"/base/bbin
# shellcheck disable=SC2068
"$GOPATH"/bin/makebb -o "$TMPDIR"/base/bbin/bb ${BB_COMMANDS[@]} || fatal "makebb: failed to create BusyBox binary"

# Create symlinks of included programs to BusyBox binary
echo "Creating links to BusyBox binary..."
# shellcheck disable=SC2068
for cmd in ${BB_COMMANDS[@]}; do
    ln -s bb "$TMPDIR"/base/bbin/"$(basename "$TMPDIR"/base/bbin/"$cmd")"
done

# copy base_dir over tmpdir if it's set
if [ -n "${BASEDIR+x}" ]; then
        echo "Overlaying ${BASEDIR}..."
        rsync -av "$BASEDIR"/ "$TMPDIR"/base
fi

echo "Creating base cpio..."
(
    cd "$TMPDIR"/base || exit 1
    find . | cpio -oc > "$TMPDIR"/base.cpio
) || fatal "Creating base cpio failed"

echo "Creating image..."
# shellcheck disable=SC2068
GOARCH="$ARCH" "$GOPATH"/bin/u-root -nocmd -initcmd=/bbin/init -uinitcmd=/bbin/uinit -defaultsh=/bbin/elvish -base "$TMPDIR"/base.cpio -o "$TMPDIR"/initramfs.cpio 2>&1

echo "CONTENTS:"
cpio -itv < "$TMPDIR"/initramfs.cpio

echo "Compressing..."
gzip "$TMPDIR"/initramfs.cpio

if [ -z "${OUTFILE+x}" ]; then
    D=$(date +%Y%m%d.%H%M)
    OUTFILE="initramfs.${D}.${ARCH}.cpio.gz"
fi
mv -v "$TMPDIR"/initramfs.cpio.gz "$PWD"/"$OUTFILE"

rm -rf "$TMPDIR"

echo "Image built as $OUTFILE"
