package service

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

type UDPPacket struct {
	content [4096]byte
	n       uint32
}

func ReadPacket(conn net.Conn, packet *UDPPacket, timeout time.Duration) error {
	conn.SetReadDeadline(time.Now().Add(timeout))
	// Read a 4-byte length prefix to determine packet size
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, lengthBuf)
	if err != nil {
		return err
	}

	// Convert to uint32 (packet length)
	packet.n = binary.BigEndian.Uint32(lengthBuf)

	// Validate packet size
	if packet.n > 4096 {
		return errors.New("invalid packet length")
	}

	if packet.n == 0 {
		return nil
	}

	_, err = io.ReadFull(conn, packet.content[:packet.n])

	if err != nil {
		return err
	}
	return nil
}

func SendPacket(conn net.Conn, packet *UDPPacket, timeout time.Duration) error {
	conn.SetWriteDeadline(time.Now().Add(timeout))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(packet.n))

	buffers := net.Buffers{lengthBuf, packet.content[:packet.n]}
	_, err := buffers.WriteTo(conn)

	if err != nil {
		return err
	}
	return nil
}
