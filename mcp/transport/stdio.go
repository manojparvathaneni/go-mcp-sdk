package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

type StdioTransport struct {
	reader       *bufio.Reader
	writer       io.Writer
	mu           sync.Mutex
	connectionID string
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		reader:       bufio.NewReader(os.Stdin),
		writer:       os.Stdout,
		connectionID: uuid.New().String(),
	}
}

func NewStdioTransportWithStreams(reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		reader:       bufio.NewReader(reader),
		writer:       writer,
		connectionID: uuid.New().String(),
	}
}

func (t *StdioTransport) Send(msg *types.JSONRPCMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	if _, err := fmt.Fprintf(t.writer, "%s\n", data); err != nil {
		return err
	}
	
	return nil
}

func (t *StdioTransport) Receive() (*types.JSONRPCMessage, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	
	var msg types.JSONRPCMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, err
	}
	
	return &msg, nil
}

func (t *StdioTransport) Close() error {
	return nil
}

func (t *StdioTransport) ConnectionID() string {
	return t.connectionID
}