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
	maxFlushBufferSize    = 32 * 1024
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
	dataChan     chan []byte
	stopOnce     sync.Once
	writeMu      sync.Mutex
	buf          bytes.Buffer
}

func newSSHTerminalSession(client *golangssh.Client, conn *websocket.Conn, rows, cols int) (*sshTerminalSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = session.Close()
		}
	}()

	modes := golangssh.TerminalModes{
		golangssh.ECHO:          1,
		golangssh.TTY_OP_ISPEED: 14400,
		golangssh.TTY_OP_OSPEED: 14400,
	}

	if err = session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		return nil, err
	}

	var stdin io.WriteCloser
	stdin, err = session.StdinPipe()
	if err != nil {
		return nil, err
	}

	var stdout io.Reader
	stdout, err = session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err = session.Shell(); err != nil {
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
		dataChan:     make(chan []byte, 32),
	}, nil
}

func (t *sshTerminalSession) Start() {
	go t.send()
	go t.output()
}

func (t *sshTerminalSession) Stop() {
	t.stopOnce.Do(func() {
		if t.cancel != nil {
			t.cancel()
		}
		if t.tick != nil {
			t.tick.Stop()
		}
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
			// 发送协议层 Ping 帧；单次超时失败不主动关闭连接，由读循环统一处理实际断开
			_ = t.sendControl(websocket.PingMessage, nil)
		case <-t.tick.C:
			if err := t.flush(); err != nil {
				t.Stop()
				return
			}
		case data := <-t.dataChan:
			t.buf.Write(data)
			// 若瞬时输出量较大（如 cat 大文件或大量滚屏），提前刷盘，避免内存积压与打字延迟
			if t.buf.Len() >= maxFlushBufferSize {
				if err := t.flush(); err != nil {
					t.Stop()
					return
				}
			}
		}
	}
}

// flush 统一抽取单点刷盘逻辑，将缓冲区数据组装为终端消息发送
func (t *sshTerminalSession) flush() error {
	if t.buf.Len() == 0 {
		return nil
	}
	msg := t.buf.String()
	t.buf.Reset()
	return t.sendMessage("stdout", msg)
}

func (t *sshTerminalSession) output() {
	buf := make([]byte, 4096)
	var leftover []byte

	for {
		// 将上次未成完整 UTF-8 字符的残余字节（最多 3 字节）保留在头部
		copy(buf, leftover)
		n, err := t.stdoutReader.Read(buf[len(leftover):])
		if err != nil {
			t.Stop()
			return
		}
		if n == 0 && len(leftover) == 0 {
			continue
		}

		var valid []byte
		valid, leftover = splitCompleteUTF8(buf[:len(leftover)+n])
		if len(valid) == 0 {
			continue
		}

		chunk := make([]byte, len(valid))
		copy(chunk, valid)

		select {
		case <-t.ctx.Done():
			return
		case t.dataChan <- chunk:
		}
	}
}

// splitCompleteUTF8 从字节流中探测并切分出完整的 UTF-8 数据块与末尾残存字节
// 严格保证中文字符（3字节）或多字节符号不被 TCP 分片截断造成乱码
func splitCompleteUTF8(data []byte) (valid []byte, leftover []byte) {
	total := len(data)
	if total == 0 {
		return nil, nil
	}

	// 从末尾最多回溯 3 字节，定位最后一个多字节字符的起始位置
	for i := total - 1; i >= 0 && i >= total-3; i-- {
		if !utf8.RuneStart(data[i]) {
			continue
		}
		if !utf8.FullRune(data[i:]) {
			return data[:i], data[i:]
		}
		break
	}
	return data, nil
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
