package main

import (
	"encoding/binary"
	"net"
)

// DecodedPacket a struct containing the layer source and destination of haproxied UDP packets
type DecodedPacket struct {
	ProtocolSignature              []byte
	ProtocolVersion                byte
	TransportProtocolAddressFamily byte
	Protocolheaderlength           uint16
	Message                        []byte
	SourceLayerAddr                net.IP
	DestinationLayerAddr           net.IP
	SourceLayerPort                uint16
	DestinationLayerPort           uint16
}

// DecodePacket Take a buffer read from a UDP network listener, and decode the layer source and destination addresses
func DecodePacket(size int, buffer []byte) DecodedPacket {
	data := buffer[:size]

	protocolSignature := data[:12]             // bytes 0-11
	protocolVersion := data[12]                // first 4 bits = version; second four bits = command (0 = LOCAL, 1 = PROXY)
	transportProtocolAddressFamily := data[13] // The highest 4 bits contain the address family, the lowest 4 bits contain the protocol.
	addressLength := data[14:16]               // address length in bytes in network endian order
	protocolheaderlength := 16 + binary.BigEndian.Uint16(addressLength)

	themessage := data[protocolheaderlength:]

	// Note: Makes an assumption that we are on IPv4 only - IPv6 addresses are 16 bytes
	// Do we use addresslength to correctly detect this?????
	sourceLayerAddr := net.IP(data[16:20]) // 197.210.29.2
	dstLayerAddr := net.IP(data[20:24])
	slaPort := data[24:26]
	dlaPort := data[26:28]

	decodedPacket := DecodedPacket{
		ProtocolSignature:              protocolSignature,
		ProtocolVersion:                protocolVersion,
		TransportProtocolAddressFamily: transportProtocolAddressFamily,
		Protocolheaderlength:           protocolheaderlength,
		Message:                        themessage,
		SourceLayerAddr:                sourceLayerAddr,
		DestinationLayerAddr:           dstLayerAddr,
		SourceLayerPort:                binary.BigEndian.Uint16(slaPort),
		DestinationLayerPort:           binary.BigEndian.Uint16(dlaPort),
	}
	return decodedPacket
}
