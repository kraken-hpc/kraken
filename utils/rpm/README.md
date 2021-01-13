# Kraken RPM building

This directory contains the necessary files to build RPMs.

The spec file, `kraken.spec`, allows for building cross-architecture RPMs with the kraken build config of your choosing.

Here's an example.  Suppose you have the kraken source in `$HOME/kraken` and you want to build an `arm64` version of the `vbox` config.

```console
$ cd $HOME/kraken
$ git archive -o ../kraken-1.0.tar.gz --prefix=kraken-1.0/ HEAD
$ rpmbuild --target aarch64-generic-linux -D 'KrakenConfig config/vbox.yaml' -D 'dist vbox' -ta ../kraken-1.0.tar.gz
```

This will build an aarch64 RPM that can be found under `$HOME/rpmbuild/RPMS/aarch64` named `kraken-1.0-0vbox.aarch64.rpm`.

If `--target` is not specified `rpmbuild` will build a native architecture build.

If you don't specify `KrakenConfig` it will default to `kraken.yaml` in the working directory.  If that file does not exist, the build will fail.

It is recommended that you specifiy `dist` to make it clear which build config kraken was based on.
