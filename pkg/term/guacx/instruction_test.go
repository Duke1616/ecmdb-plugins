package guacx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstructionSerializationAndParse(t *testing.T) {
	// 1. 基础指令
	inst := NewInstruction("size", "1024", "768", "96")
	assert.Equal(t, "4.size,4.1024,3.768,2.96;", inst.String())

	// 2. Unicode / 中文与特殊字符长度（按 CodePoint / Rune 计数）
	utfInst := NewInstruction("msg", "你好", "世界")
	// "你好" 是 2 个 rune（虽然是 6 字节），"世界" 是 2 个 rune
	assert.Equal(t, "3.msg,2.你好,2.世界;", utfInst.String())

	// 3. 参数中包含点号（如 IP、域名）不被截断解析
	ipInst := (&Instruction{}).Parse("7.connect,13.192.168.1.100,4.3389;")
	assert.Equal(t, "connect", ipInst.Opcode)
	assert.Equal(t, []string{"192.168.1.100", "3389"}, ipInst.Args)
}
