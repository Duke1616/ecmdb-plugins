package ssh

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// pipeReaderWrapper 模拟分批吐出数据的 Reader
type chunkReader struct {
	chunks [][]byte
	idx    int
}

func (c *chunkReader) Read(p []byte) (n int, err error) {
	if c.idx >= len(c.chunks) {
		return 0, io.EOF
	}
	chunk := c.chunks[c.idx]
	c.idx++
	n = copy(p, chunk)
	return n, nil
}

func TestOutputUTF8Integrity(t *testing.T) {
	// "你好世界" 的 UTF-8 编码为:
	// 你: [0xE4, 0xBD, 0xA0]
	// 好: [0xE5, 0xA5, 0xBD]
	// 世: [0xE4, 0xB8, 0x96]
	// 界: [0xE7, 0x95, 0x8C]
	fullText := "你好世界，Hello World！测试中文终端流式传输"
	fullBytes := []byte(fullText)

	// 故意在中文 3 字节中间进行分块截断拆分，模拟底层 TCP 分包
	// 第一块包含完整的 "你" + "好" 的前 2 个字节
	part1 := fullBytes[:3+2]
	part2 := fullBytes[3+2 : 12]
	part3 := fullBytes[12:]

	cr := &chunkReader{
		chunks: [][]byte{part1, part2, part3},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess := &sshTerminalSession{
		stdoutReader: bufio.NewReader(cr),
		dataChan:     make(chan []byte, 32),
		ctx:          ctx,
		cancel:       cancel,
	}

	go sess.output()

	var received bytes.Buffer
	done := time.After(2 * time.Second)

loop:
	for {
		select {
		case chunk := <-sess.dataChan:
			received.Write(chunk)
			if received.Len() >= len(fullBytes) {
				break loop
			}
		case <-done:
			t.Fatalf("读取超时，已接收字节数: %d，预期: %d", received.Len(), len(fullBytes))
		}
	}

	result := received.String()
	if result != fullText {
		t.Fatalf("UTF-8 字符重组失败:\n得到: %s\n期望: %s", result, fullText)
	}
	if strings.Contains(result, "\ufffd") {
		t.Fatalf("检测到 UTF-8 乱码字符 \\ufffd: %s", result)
	}
}

func TestSplitCompleteUTF8(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		wantValid    string
		wantLeftover []byte
	}{
		{
			name:         "空输入",
			input:        nil,
			wantValid:    "",
			wantLeftover: nil,
		},
		{
			name:         "纯 ASCII",
			input:        []byte("hello world 123"),
			wantValid:    "hello world 123",
			wantLeftover: nil,
		},
		{
			name:         "完整中文字符",
			input:        []byte("你好世界"),
			wantValid:    "你好世界",
			wantLeftover: nil,
		},
		{
			name: "截断 3 字节中文的最后 1 个字节",
			// "中" 为 [0xE4, 0xB8, 0xAD]，取前 2 字节 [0xE4, 0xB8]
			input:        append([]byte("hello "), []byte{0xE4, 0xB8}...),
			wantValid:    "hello ",
			wantLeftover: []byte{0xE4, 0xB8},
		},
		{
			name: "截断 3 字节中文的最后 2 个字节",
			// "文" 为 [0xE6, 0x96, 0x87]，取第 1 字节 [0xE6]
			input:        append([]byte("terminal "), []byte{0xE6}...),
			wantValid:    "terminal ",
			wantLeftover: []byte{0xE6},
		},
		{
			name: "截断 4 字节 Emoji (🔥: 0xF0 0x9F 0x94 0xA5)",
			// 取前 3 字节
			input:        append([]byte("echo "), []byte{0xF0, 0x9F, 0x94}...),
			wantValid:    "echo ",
			wantLeftover: []byte{0xF0, 0x9F, 0x94},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, leftover := splitCompleteUTF8(tt.input)
			if string(valid) != tt.wantValid {
				t.Errorf("splitCompleteUTF8() valid = %q, want %q", string(valid), tt.wantValid)
			}
			if !bytes.Equal(leftover, tt.wantLeftover) {
				t.Errorf("splitCompleteUTF8() leftover = %v, want %v", leftover, tt.wantLeftover)
			}
		})
	}
}


