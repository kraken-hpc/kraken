# Layer0 overview

The reference Layer0 is a u-root initramfs, with the included inito init and the correct kraken build.

The inito is responsible for reading kernel command line arguments to:

1) set the initial IP of the system;
2) start kraken with the correct (-id, -ip, -parent) options.

## Instructions for building a Layer0

Coming soon...