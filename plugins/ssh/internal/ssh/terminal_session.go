package ssh

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/Duke1616/ecmdb/pkg/term/sshx"
	"github.com/gorilla/websocket"
	golangssh "golang.org/x/crypto/ssh"
)

const (
	terminalFlushInterval = 60 * time.Millisecond
	websocketPingInterval = 25 * time.Second
	websocketWriteWait    = 5 * time.Second
)

type sshTerminalSession struct {
	client       *golangssh.Client
	conn         *websocket.Conn
	session      *golangssh.Session
	stdin        io.WriteCloser
	stdoutReader *bufio.Reader
	tick         *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
	dataChan     chan rune
	stopOnce     sync.Once
	writeMu      sync.Mutex
	buf          bytes.Buffer
}

func newSSHTerminalSession(client *golangssh.Client, conn *websocket.Conn, rows, cols int) (*sshTerminalSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	modes := golangssh.TerminalModes{
		golangssh.ECHO:          1,
		golangssh.TTY_OP_ISPEED: 14400,
		golangssh.TTY_OP_OSPEED: 14400,
	}

	if err = session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}

	if err = session.Shell(); err != nil {
		_ = session.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &sshTerminalSession{
		client:       client,
		conn:         conn,
		session:      session,
		stdin:        stdin,
		stdoutReader: bufio.NewReader(stdout),
		tick:         time.NewTicker(terminalFlushInterval),
		ctx:          ctx,
		cancel:       cancel,
		dataChan:     make(chan rune),
	}, nil
}

func (t *sshTerminalSession) Start() {
	go t.send()
	go t.output()
}

func (t *sshTerminalSession) Stop() {
	t.stopOnce.Do(func() {
		t.cancel()
		t.tick.Stop()
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.session != nil {
			_ = t.session.Close()
		}
		if t.conn != nil {
			_ = t.conn.Close()
		}
	})
}

func (t *sshTerminalSession) Resize(rows, cols int) error {
	return t.session.WindowChange(rows, cols)
}

func (t *sshTerminalSession) Write(data []byte) error {
	_, err := t.stdin.Write(data)
	return err
}

func (t *sshTerminalSession) Ping() error {
	if err := t.keepSSHAlive(); err != nil {
		return err
	}
	return t.sendMessage("pong", "")
}

func (t *sshTerminalSession) keepSSHAlive() error {
	if t.client != nil {
		if _, _, err := t.client.Conn.SendRequest(sshKeepaliveRequest, false, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *sshTerminalSession) send() {
	defer t.buf.Reset()
	pingTicker := time.NewTicker(websocketPingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-pingTicker.C:
			if err := t.keepSSHAlive(); err != nil {
				t.Stop()
				return
			}
			if err := t.sendControl(websocket.PingMessage, nil); err != nil {
				t.Stop()
				return
			}
		case <-t.tick.C:
			msg := t.buf.String()
			if msg == "" {
				continue
			}
			if err := t.sendMessage("stdout", msg); err != nil {
				t.Stop()
				return
			}
			t.buf.Reset()
		case data := <-t.dataChan:
			p := make([]byte, utf8.RuneLen(data))
			utf8.EncodeRune(p, data)
			t.buf.Write(p)
		}
	}
}

func (t *sshTerminalSession) output() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			rn, size, err := t.stdoutReader.ReadRune()
			if err != nil {
				t.Stop()
				return
			}
			if size == 0 || rn == utf8.RuneError {
				continue
			}

			select {
			case <-t.ctx.Done():
				return
			case t.dataChan <- rn:
			}
		}
	}
}

func (t *sshTerminalSession) sendMessage(operation, data string) error {
	message, err := json.Marshal(sshx.NewMessage(operation, data, 0, 0))
	if err != nil {
		return err
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, message)
}

func (t *sshTerminalSession) sendControl(messageType int, data []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.conn.WriteControl(messageType, data, time.Now().Add(websocketWriteWait))
}

var _ term.TerminalSession = (*sshTerminalSession)(nil)
