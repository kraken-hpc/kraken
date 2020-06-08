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
        echo "Usage: $0 [-o <out_file>] [-b <base_dir>] <kraken_build_dir> <arch>"
        echo "  <arch> should be the GOARCH we want to build (e.g. arm64, amd64...)"
        echo "  <out_file> is the file the image should be written to.  (default: initramfs.<date>.<img>.cpio.gz)"
        echo "  <base_dir> is an optional base directory containing file/directory structure (default: none)"
        echo "             that should be added to the image"
        echo "  <kraken_build_dir> is specifies where to look for generated kraken source tree for u-root"
}

opts=$(getopt o:b: $*)
if [ $? != 0 ]; then
    usage
    exit
fi

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
        --)
            shift; break;;
    esac
done

if [ $# -ne 2 ]; then
    usage
    exit 1 
fi 

STARTDIR=$PWD
KRAKEN_BUILDDIR=$1
ARCH=$2
TMPDIR=$(mktemp -d)

if [ -z ${GOPATH+x} ]; then
    echo "GOPATH isn't set, using $HOME/go"
    GOPATH=$HOME/go
fi

if [ -z ${KRAKEN_BUILDDIR+x} ]; then
    echo "No kraken_build_dir specified. I need it to include in initramfs!"
    usage
    exit 1
fi

# Check that kraken build dir is not empty
if [ -z "$(ls -A $KRAKEN_BUILDDIR)" ]; then
    echo "$KRAKEN_BUILDDIR is empty; build it before running this"
    exit 1
fi
echo "Using generated kraken source tree at $KRAKEN_BUILDDIR"

# copy base_dir over tmpdir if it's set
if [ ! -z ${BASEDIR+x} ]; then 
        echo "Overlaying ${BASEDIR}..."
        rsync -av $BASEDIR/ $TMPDIR/base
fi

echo "Creating base cpio..."
(
    cd $TMPDIR/base
    find . | cpio -oc > $TMPDIR/base.cpio
)

# Check that u-root is installed, clone it if not
if [ ! -x $GOPATH/bin/u-root ]; then
    echo "You don't appear to have u-root installed, attempting to install it"
    GOPATH=$GOPATH go get github.com/u-root/u-root
fi
echo "Creating image..."
GOARCH=$ARCH $GOPATH/bin/u-root -base $TMPDIR/base.cpio -build bb -o $TMPDIR/initramfs.cpio "$KRAKEN_BUILDDIR"/u-root/kraken "${KRAKEN_SOURCEDIR}/utils/layer0/uinit"

echo "CONTENTS:"
cpio -itv < $TMPDIR/initramfs.cpio

echo "Compressing..."
gzip $TMPDIR/initramfs.cpio

if [ -z ${OUTFILE+x} ]; then 
    D=$(date +%Y%m%d.%H%M)
    OUTFILE="initramfs.${D}.${ARCH}.cpio.gz"
fi
mv -v $TMPDIR/initramfs.cpio.gz $PWD/$OUTFILE

rm -rf $TMPDIR

echo "Image built as $OUTFILE"
