//go:build windows

package host

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func (h *Real) ensureElevated() error {
	if h.elevated && h.proxy != nil {
		return nil
	}
	if h.elevated {
		return nil
	}
	if isAdmin() {
		h.elevated = true
		return nil
	}
	client, err := startElevatedHelper()
	if err != nil {
		return err
	}
	h.proxy = client
	h.elevated = true
	return nil
}

func isAdmin() bool {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("IsUserAnAdmin")
	r, _, _ := proc.Call()
	return r != 0
}

func startElevatedHelper() (*elevatedClient, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot find this program to elevate: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	tokBytes := make([]byte, 16)
	if _, err := rand.Read(tokBytes); err != nil {
		ln.Close()
		return nil, err
	}
	tok := hex.EncodeToString(tokBytes)
	addr := ln.Addr().String()

	ps := fmt.Sprintf(
		`Start-Process -FilePath '%s' -ArgumentList @('--install-helper','%s','%s') -Verb RunAs`,
		strings.ReplaceAll(exe, "'", "''"),
		strings.ReplaceAll(addr, "'", "''"),
		tok,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
	if err := cmd.Start(); err != nil {
		ln.Close()
		return nil, fmt.Errorf("could not request Windows admin: %w", err)
	}
	_ = cmd.Process.Release()

	ln.(*net.TCPListener).SetDeadline(time.Now().Add(3 * time.Minute))
	conn, err := ln.Accept()
	ln.Close()
	if err != nil {
		return nil, fmt.Errorf("admin prompt was declined or timed out")
	}
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	var hello helperRequest
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := dec.Decode(&hello); err != nil {
		conn.Close()
		return nil, fmt.Errorf("elevated helper did not connect")
	}
	if hello.Op != "hello" || hello.Tok != tok {
		conn.Close()
		return nil, fmt.Errorf("elevated helper token mismatch")
	}
	conn.SetDeadline(time.Time{})
	if err := enc.Encode(helperResponse{OK: true}); err != nil {
		conn.Close()
		return nil, err
	}
	return &elevatedClient{conn: conn, enc: enc, dec: dec}, nil
}

func RunInstallHelper(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("install helper needs address and token")
	}
	addr, tok := args[0], args[1]
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(helperRequest{Op: "hello", Tok: tok}); err != nil {
		return err
	}
	var ack helperResponse
	if err := dec.Decode(&ack); err != nil {
		return err
	}
	if !ack.OK {
		return fmt.Errorf("parent rejected helper")
	}
	local := NewReal()
	local.elevated = true
	for {
		var req helperRequest
		if err := dec.Decode(&req); err != nil {
			return nil
		}
		var runErr error
		switch req.Op {
		case "exit":
			_ = enc.Encode(helperResponse{OK: true})
			return nil
		case "start":
			runErr = local.StartWait(req.Path, req.Args)
		case "kind":
			runErr = local.InstallKind(req.Kind, req.Path, req.Args)
		default:
			runErr = fmt.Errorf("unknown helper op %s", req.Op)
		}
		resp := helperResponse{OK: runErr == nil}
		if runErr != nil {
			resp.Err = runErr.Error()
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
}
