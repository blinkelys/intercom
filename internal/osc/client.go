package osc

import (
	"bytes"
	"fmt"
	"net"
)

// Message represents one outbound OSC dispatch.
type Message struct {
	Address   string
	Arguments []string
}

// Sender sends OSC messages to a destination.
type Sender interface {
	Send(Message) error
}

// UDPClient sends OSC messages over UDP.
type UDPClient struct {
	address string
}

// NewUDPClient constructs a UDP OSC sender.
func NewUDPClient(destination string, port int) (*UDPClient, error) {
	if destination == "" {
		return nil, fmt.Errorf("osc destination is required")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("osc port %d is out of range", port)
	}
	return &UDPClient{address: fmt.Sprintf("%s:%d", destination, port)}, nil
}

// Send encodes and transmits one OSC message.
func (c *UDPClient) Send(message Message) error {
	payload, err := Encode(message)
	if err != nil {
		return err
	}
	conn, err := net.Dial("udp", c.address)
	if err != nil {
		return fmt.Errorf("dial osc destination: %w", err)
	}
	defer conn.Close()
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write osc payload: %w", err)
	}
	return nil
}

// Encode converts a message to an OSC wire payload.
func Encode(message Message) ([]byte, error) {
	if message.Address == "" || message.Address[0] != '/' {
		return nil, fmt.Errorf("osc address must start with '/'")
	}

	buffer := bytes.NewBuffer(nil)
	writePaddedString(buffer, message.Address)

	typeTags := ","
	for range message.Arguments {
		typeTags += "s"
	}
	writePaddedString(buffer, typeTags)

	for _, argument := range message.Arguments {
		writePaddedString(buffer, argument)
	}

	return buffer.Bytes(), nil
}

func writePaddedString(buffer *bytes.Buffer, value string) {
	buffer.WriteString(value)
	buffer.WriteByte(0)
	for buffer.Len()%4 != 0 {
		buffer.WriteByte(0)
	}
}
