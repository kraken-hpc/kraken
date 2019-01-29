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

if [ $# -lt 1 ]; then
        echo "Usage: $0 <arch> [<base_dir>]"
        echo "  <arch> should be the GOARCH we want to build (e.g. arm64, amd64...)"
        echo "  <base_dir> is an option base directory containing file/directory structure"
        echo "             that should be added to the image"
        exit
fi

STARTDIR=$PWD
ARCH=$1

if [ $# -eq 2 ]; then
        BASEDIR=$2
fi


if [ -z ${GOPATH+x} ]; then
        echo "GOPATH isn't set, using $HOME/go"
        GOPATH=$HOME/go
fi

# make a temporary directory for our base
TMPDIR=$(mktemp -d)
echo "Using tmpdir: $TMPDIR"
mkdir -p $TMPDIR/base/bin

KRAKEN=$GOPATH/src/github.com/hpc/kraken/build/kraken-linux-$ARCH
if [ ! -f $KRAKEN ]; then
    echo "$KARKEN doesn't exist, built it before running this"
    rm -rf $TMPDIR
    exit
fi
echo "Using $KRAKEN"
cp -v $KRAKEN $TMPDIR/base/bin/kraken

# make uinit & dssh
(
    cd $TMPDIR/base/bin
    echo "Build dssh..."
    GOARCH=$ARCH CGO_ENABLED=0 go build $GOPATH/src/github.com/hpc/kraken/utils/layer0/dssh/dssh.go
    echo "Build uinit..."
    GOARCH=$ARCH CGO_ENABLED=0 go build $GOPATH/src/github.com/hpc/kraken/utils/layer0/uinit/uinit.go
)

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

if [ ! -x $GOPATH/bin/u-root ]; then
    echo "You don't appear to have u-root installed, attempting to install it"
    GOPATH=$GOPATH go install github.com/u-root/u-root
fi
echo "Creating image..."
GOARCH=$ARCH $GOPATH/bin/u-root -base $TMPDIR/base.cpio -build bb -o $TMPDIR/initramfs.cpio

echo "Compressing..."
gzip $TMPDIR/initramfs.cpio

D=$(date +%Y%m%d.%H%M)
mv -v $TMPDIR/initramfs.cpio.gz $PWD/initramfs.${D}.${ARCH}.cpio.gz

rm -rf $TMPDIR

echo "Image built as initramfs.${D}.${ARCH}.cpio.gz"