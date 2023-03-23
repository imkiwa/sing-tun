//go:build with_gvisor

package tun

import (
	"github.com/sagernet/sing/common/buf"

	"github.com/metacubex/gvisor/pkg/tcpip/stack"
)

type DirectDestination interface {
	WritePacket(buffer *buf.Buffer) error
	WritePacketBuffer(buffer stack.PacketBufferPtr) error
	Close() error
	Timeout() bool
}