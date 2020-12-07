package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	isServer = flag.Bool("server", false, "whether it should be run as a server")
	port     = flag.Uint("port", 8081, "port to send to or receive from")
	host     = flag.String("host", "0.0.0.0", "address to send to or receive from")
	timeout  = flag.Duration("timeout", 15*time.Second, "read and write blocking deadlines")
	input    = flag.String("input", "-", "file with contents to send over udp")
)

// maxBufferSize specifies the size of the buffers that
// are used to temporarily hold data from the UDP packets
// that we receive.
const maxBufferSize = 1024

// server wraps all the UDP echo server functionality.
// ps.: the server is capable of answering to a single
// client at a time.
func server(ctx context.Context, address string) (err error) {
	// ListenPacket provides us a wrapper around ListenUDP so that
	// we don't need to call `net.ResolveUDPAddr` and then subsequentially
	// perform a `ListenUDP` with the UDP address.
	//
	// The returned value (PacketConn) is pretty much the same as the one
	// from ListenUDP (UDPConn) - the only difference is that `Packet*`
	// methods and interfaces are more broad, also covering `ip`.
	pc, err := net.ListenPacket("udp", address)
	if err != nil {
		return
	}

	// addr := pc.LocalAddr()
	// `Close`ing the packet "connection" means cleaning the data structures
	// allocated for holding information about the listening socket.
	defer pc.Close()

	doneChan := make(chan error, 1)
	buffer := make([]byte, maxBufferSize)
	f, err := os.OpenFile("./messages.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("error opening file: %s", err)
	}
	// Given that waiting for packets to arrive is blocking by nature and we want
	// to be able of cancelling such action if desired, we do that in a separate
	// go routine.
	go func() {
		for {
			// By reading from the connection into the buffer, we block until there's
			// new content in the socket that we're listening for new packets.
			//
			// Whenever new packets arrive, `buffer` gets filled and we can continue
			// the execution.
			//
			// note.: `buffer` is not being reset between runs.
			//	  It's expected that only `n` reads are read from it whenever
			//	  inspecting its contents.
			size, addr, err := pc.ReadFrom(buffer)
			if err != nil {
				doneChan <- err
				return
			}
			decodedPacket := DecodePacket(size, buffer)

			var b strings.Builder
			b.WriteString(fmt.Sprintf("Protocol Signature: %#v\n", decodedPacket.ProtocolSignature))
			b.WriteString(fmt.Sprintf("Protocol Version: %#v\n", decodedPacket.ProtocolVersion))
			b.WriteString(fmt.Sprintf("Transport Protocol/Address family: %#v\n", decodedPacket.TransportProtocolAddressFamily))
			b.WriteString(fmt.Sprintf("sourceLayerAddr: %v\n", decodedPacket.SourceLayerAddr))
			b.WriteString(fmt.Sprintf("dstLayerAddr: %v\n", decodedPacket.DestinationLayerAddr))
			b.WriteString(fmt.Sprintf("slaPort: %v\n", decodedPacket.SourceLayerPort))
			b.WriteString(fmt.Sprintf("dlaPort: %v\n", decodedPacket.DestinationLayerPort))
			b.WriteString(fmt.Sprintf("packet-received: bytes=%d from=%s message:%s\n",
				size, addr.String(), string(decodedPacket.Message)))
			fmt.Println(b.String())
			// write the message to log file
			f.Write([]byte(b.String()))
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("cancelled")
		f.Close()
		err = ctx.Err()
	case err = <-doneChan:
	}

	return
}

// client wraps the whole functionality of a UDP client that sends
// a message and waits for a response coming back from the server
// that it initially targetted.
func client(ctx context.Context, address string, reader io.Reader) (err error) {
	// Resolve the UDP address so that we can make use of DialUDP
	// with an actual IP and port instead of a name (in case a
	// hostname is specified).
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}

	// Although we're not in a connection-oriented transport,
	// the act of `dialing` is analogous to the act of performing
	// a `connect(2)` syscall for a socket of type SOCK_DGRAM:
	// - it forces the underlying socket to only read and write
	//   to and from a specific remote address.
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return
	}

	// Closes the underlying file descriptor associated with the,
	// socket so that it no longer refers to any file.
	defer conn.Close()

	doneChan := make(chan error, 1)

	go func() {
		// It is possible that this action blocks, although this
		// should only occur in very resource-intensive situations:
		// - when you've filled up the socket buffer and the OS
		//   can't dequeue the queue fast enough.
		n, err := io.Copy(conn, reader)
		if err != nil {
			doneChan <- err
			return
		}

		fmt.Printf("packet-written: bytes=%d\n", n)

		// Q: How can we make sure that we're reading all that
		//    we want? e.g., what's the best way of making sure
		//    that we were able to consume the whole msg that is
		//    already in the queue?
		//
		// Q: What happens if we play around with the rcv/wrt
		//    buffer size?
		buffer := make([]byte, maxBufferSize)

		// Set a deadline for the ReadOperation so that we don't
		// wait forever for a server that might not respond on
		// a resonable amount of time.
		deadline := time.Now().Add(*timeout)
		err = conn.SetReadDeadline(deadline)
		if err != nil {
			doneChan <- err
			return
		}

		nRead, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			doneChan <- err
			return
		}

		fmt.Printf("packet-received: bytes=%d from=%s\n",
			nRead, addr.String())

		doneChan <- nil
	}()

	select {
	case <-ctx.Done():
		fmt.Println("cancelled")
		err = ctx.Err()
	case err = <-doneChan:
	}

	return
}

func main() {
	flag.Parse()

	var (
		err     error
		address = fmt.Sprintf("%s:%d", *host, *port)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Gracefully handle signals so that we can finalize any of our
	// blocking operations by cancelling their contexts.
	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	if *isServer {
		fmt.Println("running as a server on " + address)
		err = server(ctx, address)
		if err != nil && err != context.Canceled {
			panic(err)
		}
		return
	}

	// allow the client to receive the contents to be sent
	// either via `stdin` or via a file that can be supplied
	// via the `-input=` flag.
	reader := os.Stdin
	if *input != "-" {
		file, err := os.Open(*input)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		reader = file
	}

	fmt.Println("sending to " + address)
	err = client(ctx, address, reader)
	if err != nil && err != context.Canceled {
		panic(err)
	}
}
