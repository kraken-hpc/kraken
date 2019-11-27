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
        echo "  <kraken_build_dir> is specifies an alternate path to where to look for built kraken binaries (default: kraken_source_dir/build)"
}

opts=$(getopt o:b:k:B: $*)
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

STARTDIR=$PWD
ARCH=$1

if [ -z ${GOPATH+x} ]; then
    echo "GOPATH isn't set, using $HOME/go"
    GOPATH=$HOME/go
fi

if [ -z ${KRAKEN_SOURCEDIR+x} ]; then
    KRAKEN_SOURCEDIR="$GOPATH/src/github.com/hpc/kraken"
    echo "Using kraken source dir $KRAKEN_SOURCEDIR"
fi

if [ -z ${KRAKEN_BUILDDIR+x} ]; then
    KRAKEN_BUILDDIR="$KRAKEN_SOURCEDIR/build"
    echo "Using kraken build dir $KRAKEN_BUILDDIR"
fi

# make a temporary directory for our base
TMPDIR=$(mktemp -d)
echo "Using tmpdir: $TMPDIR"
mkdir -p $TMPDIR/base/bin

KRAKEN="$KRAKEN_BUILDDIR/kraken-linux-$ARCH"
if [ ! -f $KRAKEN ]; then
    echo "$KRAKEN doesn't exist, built it before running this"
    rm -rf $TMPDIR
    exit
fi
echo "Using $KRAKEN"
cp -v $KRAKEN $TMPDIR/base/bin/kraken

# make pre-requisite binaries
(
    cd $TMPDIR/base/bin
    echo "Build uinit..."
    GOARCH=$ARCH CGO_ENABLED=0 go build "${KRAKEN_SOURCEDIR}/utils/layer0/uinit/uinit.go"
    
    echo "Build rfemulator..."
    GOARCH=$ARCH CGO_ENABLED=0 go build $GOPATH/src/github.com/hpc/kraken/utils/layer0/rfemulator/RFEmulator-pull.go
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
    GOPATH=$GOPATH go get github.com/u-root/u-root
fi
echo "Creating image..."
GOARCH=$ARCH $GOPATH/bin/u-root -base $TMPDIR/base.cpio -build bb -o $TMPDIR/initramfs.cpio

echo "Compressing..."
gzip $TMPDIR/initramfs.cpio

if [ -z ${OUTFILE+x} ]; then 
    D=$(date +%Y%m%d.%H%M)
    OUTFILE="initramfs.${D}.${ARCH}.cpio.gz"
fi
mv -v $TMPDIR/initramfs.cpio.gz $PWD/$OUTFILE

rm -rf $TMPDIR

echo "Image built as $OUTFILE"
