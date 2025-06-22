package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

type SSETransport struct {
	writer       http.ResponseWriter
	flusher      http.Flusher
	reader       *bufio.Reader
	mu           sync.Mutex
	connectionID string
	messageChan  chan *types.JSONRPCMessage
	closed       bool
}

func NewSSETransport(w http.ResponseWriter, r *http.Request) (*SSETransport, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	transport := &SSETransport{
		writer:       w,
		flusher:      flusher,
		reader:       bufio.NewReader(r.Body),
		connectionID: uuid.New().String(),
		messageChan:  make(chan *types.JSONRPCMessage, 100),
	}
	
	go transport.readLoop()
	
	return transport, nil
}

func (t *SSETransport) Send(msg *types.JSONRPCMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return fmt.Errorf("transport closed")
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	fmt.Fprintf(t.writer, "event: message\n")
	fmt.Fprintf(t.writer, "data: %s\n\n", data)
	t.flusher.Flush()
	
	return nil
}

func (t *SSETransport) Receive() (*types.JSONRPCMessage, error) {
	msg, ok := <-t.messageChan
	if !ok {
		return nil, fmt.Errorf("transport closed")
	}
	return msg, nil
}

func (t *SSETransport) readLoop() {
	defer close(t.messageChan)
	
	scanner := bufio.NewScanner(t.reader)
	var eventType string
	var dataBuffer bytes.Buffer
	
	for scanner.Scan() {
		line := scanner.Text()
		
		if line == "" {
			if eventType == "message" && dataBuffer.Len() > 0 {
				var msg types.JSONRPCMessage
				if err := json.Unmarshal(dataBuffer.Bytes(), &msg); err == nil {
					t.messageChan <- &msg
				}
				dataBuffer.Reset()
				eventType = ""
			}
			continue
		}
		
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			dataBuffer.WriteString(strings.TrimSpace(data))
		}
	}
}

func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.closed {
		t.closed = true
		fmt.Fprintf(t.writer, "event: close\n\n")
		t.flusher.Flush()
	}
	
	return nil
}

func (t *SSETransport) ConnectionID() string {
	return t.connectionID
}