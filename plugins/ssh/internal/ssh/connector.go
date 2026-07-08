package ssh

import (
	"context"
	"fmt"
	"net"

	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	golangssh "golang.org/x/crypto/ssh"
)

type sshConnector struct{}

const sshKeepaliveRequest = "keepalive@openssh.com"

func (s *sshConnector) Name() string {
	return "ssh"
}

func (s *sshConnector) Connect(ctx context.Context, chain term.GatewayChain, opts term.ConnectOptions) (term.Session, error) {
	builder := NewSSHChainBuilder()
	transport, err := builder.Build(chain)
	if err != nil {
		return nil, err
	}

	chainTransport, ok := transport.(*sshChainTransport)
	if !ok {
		return nil, fmt.Errorf("unexpected transport type for ssh connector")
	}
	if err = chainTransport.ensureClient(ctx); err != nil {
		return nil, err
	}

	return &sshSession{client: chainTransport.client}, nil
}

type sshSession struct {
	client *golangssh.Client
}

func (s *sshSession) Protocol() string {
	return "ssh"
}

func (s *sshSession) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *sshSession) Transport() term.Transport {
	return &sshTransport{client: s.client}
}

type sshTransport struct {
	client *golangssh.Client
}

func (t *sshTransport) Dial(ctx context.Context, ep term.Endpoint) (net.Conn, error) {
	if t.client == nil {
		return nil, fmt.Errorf("ssh transport client is nil")
	}
	address := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
	return t.client.DialContext(ctx, "tcp", address)
}

func (s *sshSession) NewTerminal(ws *websocket.Conn, rows, cols int) (term.TerminalSession, error) {
	return newSSHTerminalSession(s.client, ws, rows, cols)
}

func (s *sshSession) NewSFTP() (*sftp.Client, error) {
	return sftp.NewClient(s.client)
}

var _ term.Session = (*sshSession)(nil)
var _ term.ShellCapable = (*sshSession)(nil)
var _ term.FileCapable = (*sshSession)(nil)

func init() {
	term.RegisterConnector(&sshConnector{})
}
