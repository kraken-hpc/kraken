#!/bin/bash

# This script creates a config that includes all available modules and extensions
# It will create build targets as configured in this script
# This is mostly intended for running tests

# This takes the root of the kraken source directory as the first (and only) argument

if [ $# -ne 1 ]; then
  echo "Usage: $0 <kraken_src_dir>"
  exit 1
fi

KRAKEN_SRC="$1"

PKG="github.com/kraken-hpc/kraken"
CONFIG_FILE="$KRAKEN_SRC/config/kitchensink.yaml"
EXTENSIONS_DIR="$KRAKEN_SRC/extensions"
MODULES_DIR="$KRAKEN_SRC/modules"

echo "Adding build targets"
META="
name: kitchen-sink
version: v0.0.0
"

# start with our build targets
echo "$META" > "$CONFIG_FILE"

# include extensions
echo "Adding extensions"
echo "extensions: " >> "$CONFIG_FILE"
for e_dir in "$EXTENSIONS_DIR"/*; do 
  if [ ! -d "$e_dir" ]; then
    continue 
  fi
  e=$(basename "$e_dir")
  echo "Adding extension: $e"
  echo "  - $PKG/extensions/$e" >> "$CONFIG_FILE"
done

# include modules
echo "Adding modules"
echo "modules: " >> "$CONFIG_FILE"
for m_dir in "$MODULES_DIR"/*; do 
  if [ ! -d "$m_dir" ]; then
    continue 
  fi
  m=$(basename "$m_dir")
  echo "Adding module: $m"
  echo "  - $PKG/modules/$m" >> "$CONFIG_FILE"
done

echo "created kitchen sink config: $CONFIG_FILE"