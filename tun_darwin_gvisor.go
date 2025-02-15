//go:build with_gvisor && darwin

package tun

import (
	"github.com/sagernet/sing/common/bufio"

	"github.com/metacubex/gvisor/pkg/buffer"
	"github.com/metacubex/gvisor/pkg/tcpip"
	"github.com/metacubex/gvisor/pkg/tcpip/header"
	"github.com/metacubex/gvisor/pkg/tcpip/stack"
)

var _ GVisorTun = (*NativeTun)(nil)

func (t *NativeTun) NewEndpoint() (stack.LinkEndpoint, error) {
	return &DarwinEndpoint{tun: t}, nil
}

var _ stack.LinkEndpoint = (*DarwinEndpoint)(nil)

type DarwinEndpoint struct {
	tun        *NativeTun
	dispatcher stack.NetworkDispatcher
}

func (e *DarwinEndpoint) MTU() uint32 {
	return e.tun.mtu
}

func (e *DarwinEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (e *DarwinEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (e *DarwinEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

func (e *DarwinEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	if dispatcher == nil && e.dispatcher != nil {
		e.dispatcher = nil
		return
	}
	if dispatcher != nil && e.dispatcher == nil {
		e.dispatcher = dispatcher
		go e.dispatchLoop()
	}
}

func (e *DarwinEndpoint) dispatchLoop() {
	packetBuffer := make([]byte, e.tun.mtu+4)
	for {
		n, err := e.tun.tunFile.Read(packetBuffer)
		if err != nil {
			break
		}
		packet := packetBuffer[4:n]
		var networkProtocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(packet) {
		case header.IPv4Version:
			networkProtocol = header.IPv4ProtocolNumber
			if header.IPv4(packet).DestinationAddress().As4() == e.tun.inet4Address {
				e.tun.tunFile.Write(packetBuffer[:n])
				continue
			}
		case header.IPv6Version:
			networkProtocol = header.IPv6ProtocolNumber
			if header.IPv6(packet).DestinationAddress().As16() == e.tun.inet6Address {
				e.tun.tunFile.Write(packetBuffer[:n])
				continue
			}
		default:
			e.tun.tunFile.Write(packetBuffer[:n])
			continue
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload:           buffer.MakeWithData(packetBuffer[4:n]),
			IsForwardedPacket: true,
		})
		pkt.NetworkProtocolNumber = networkProtocol
		dispatcher := e.dispatcher
		if dispatcher == nil {
			pkt.DecRef()
			return
		}
		dispatcher.DeliverNetworkPacket(networkProtocol, pkt)
		pkt.DecRef()
	}
}

func (e *DarwinEndpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *DarwinEndpoint) Wait() {
}

func (e *DarwinEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *DarwinEndpoint) AddHeader(buffer stack.PacketBufferPtr) {
}

func (e *DarwinEndpoint) WritePackets(packetBufferList stack.PacketBufferList) (int, tcpip.Error) {
	var n int
	for _, packet := range packetBufferList.AsSlice() {
		var packetHeader []byte
		switch packet.NetworkProtocolNumber {
		case header.IPv4ProtocolNumber:
			packetHeader = packetHeader4[:]
		case header.IPv6ProtocolNumber:
			packetHeader = packetHeader6[:]
		}
		_, err := bufio.WriteVectorised(e.tun.tunWriter, append([][]byte{packetHeader}, packet.AsSlices()...))
		if err != nil {
			return n, &tcpip.ErrAborted{}
		}
		n++
	}
	return n, nil
}
