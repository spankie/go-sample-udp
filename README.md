# UDP Packet Decoding Utility

WIP

HAProxy UDP packet decoder.

See main.go for usage at the moment. Still needs refactoring.


## Build

To build:

```
make build
```

If you get an error `/usr/local/go/pkg/darwin_amd64/runtime/cgo.a: permission denied`, see [here](https://stackoverflow.com/questions/60771344/go-build-i-cause-open-usr-local-go-pkg-darwin-amd64-runtime-cgo-a-permission).

## Usage:

	1. retrieve the code and install it

		go get github.com/cirocosta/go-sample-udp

	2. open a terminal and start a server

		go-sample-udp -server

	3. open another terminal and make the client send a message

		echo "my-message" | go-sample-udp


More:
	https://ops.tips
