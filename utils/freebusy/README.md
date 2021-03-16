# freebusy

`freebusy` is a utility to set free/busy states on a local node. 

It expects `kraken.id=<id>` to exist in `/proc/cmdline`.

`freebusy` communicates via the Module API (gRPC).

# building

```
cd $KARKEN_SRC/util/freebusy
CGO_ENABLED=0 go build .
```

Alternatively, `freebusy` can be built into the `u-root` busybox.

# using

On the local node, run:

```
freebusy <free|busy>
```
