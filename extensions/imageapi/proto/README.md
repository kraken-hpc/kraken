# ImageAPI Protobuf

The file `generated.proto` is generated directly from the ImageAPI swagger definition.  We then extend it with imageapi.proto.

To generate:
```bash
$ go get -u github.com/NYTimes/openapi2proto/cmd/openapi2proto
$ openapi2proto -indent 2 -out generated.proto -skip-rpcs -spec /path/to/swagger.yaml
```

Finally, we need to replace the generated file headers to contain:

```proto
// BEGIN: NON-GENERATED HEADER
syntax = "proto3";
package ImageAPI;

option go_package = ".;imageapi";

import "github.com/gogo/protobuf/gogoproto/gogo.proto";
option (gogoproto.marshaler_all) = true;
option (gogoproto.unmarshaler_all) = true;
option (gogoproto.sizer_all) = true;
option (gogoproto.goproto_registration) = true;
option (gogoproto.messagename_all) = true;


message CustomType {
  RbdOptions secret = 1 [(gogoproto.customtype) = "github.com/hpc/kraken/extensions/imageapi/customtypes.Secret"];
}
// END: NON-GENERATED HEADER
// AUTO-GENERATED CONTENT BELOW
```