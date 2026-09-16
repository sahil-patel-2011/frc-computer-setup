package host

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type helperRequest struct {
	Op   string   `json:"op"`
	Kind string   `json:"kind,omitempty"`
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
	Tok  string   `json:"tok,omitempty"`
}

type helperResponse struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}

type elevatedClient struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	mu   sync.Mutex
}

func (c *elevatedClient) call(req helperRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.enc.Encode(req); err != nil {
		return err
	}
	var resp helperResponse
	if err := c.dec.Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Err == "" {
			return fmt.Errorf("elevated helper failed")
		}
		return fmt.Errorf("%s", resp.Err)
	}
	return nil
}

func (c *elevatedClient) StartWait(path string, args []string) error {
	return c.call(helperRequest{Op: "start", Path: path, Args: args})
}

func (c *elevatedClient) InstallKind(kind, path string, args []string) error {
	return c.call(helperRequest{Op: "kind", Kind: kind, Path: path, Args: args})
}

func (c *elevatedClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.enc.Encode(helperRequest{Op: "exit"})
	return c.conn.Close()
}
