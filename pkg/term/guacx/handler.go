package guacx

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
)

type GuacamoleHandler struct {
	ws       *websocket.Conn
	tunnel   *Tunnel
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

func NewGuacamoleHandler(ws *websocket.Conn, tunnel *Tunnel) *GuacamoleHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &GuacamoleHandler{
		ws:     ws,
		tunnel: tunnel,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动后台读取并向 WebSocket 转发 Guacamole 指令流
func (r *GuacamoleHandler) Start() {
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				return
			default:
				instruction, err := r.tunnel.Read()
				if err != nil {
					return
				}
				if len(instruction) == 0 {
					continue
				}
				err = r.ws.WriteMessage(websocket.TextMessage, instruction)
				if err != nil {
					return
				}
			}
		}
	}()
}

// Stop 停止转发并释放底层的 Guacamole Tunnel 连接
func (r *GuacamoleHandler) Stop() {
	r.stopOnce.Do(func() {
		r.cancel()
		if r.tunnel != nil {
			_ = r.tunnel.Close()
		}
	})
}
