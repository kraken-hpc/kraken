`vboxapi.go` provides a simple rest api that mimicks the Node.js project [vboxmanage-rest-api](https://www.npmjs.com/package/vboxmanage-rest-api) for only the bits Kraken needs.

```bash
$ vboxapi -h
Usage of vboxapi:
  -base string
        base URL for api (default "/vboxmanage")
  -ip string
        ip to listen on (default "127.0.0.1")
  -port uint
        port to listen on (default 8269)
  -v    verbose messages
  -vbm string
        full path to vboxmanage command (default "/usr/local/bin/vboxmanage")
```