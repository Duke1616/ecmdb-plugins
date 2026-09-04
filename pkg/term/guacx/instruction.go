package guacx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const delimiter = ';'

type Instruction struct {
	Opcode string
	Args   []string
	cache  string
}

func NewInstruction(opcode string, args ...string) *Instruction {
	return &Instruction{
		Opcode: opcode,
		Args:   args,
	}
}

// String 序列化 Guacamole 指令
func (i *Instruction) String() string {
	if len(i.cache) > 0 {
		return i.cache
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%d.%s", utf8.RuneCountInString(i.Opcode), i.Opcode))
	for _, value := range i.Args {
		builder.WriteByte(',')
		builder.WriteString(fmt.Sprintf("%d.%s", utf8.RuneCountInString(value), value))
	}
	builder.WriteRune(delimiter)

	i.cache = builder.String()
	return i.cache
}

func (i *Instruction) Bytes() []byte {
	return []byte(i.String())
}

// Parse 解析单条 Guacamole 指令字符串
func (i *Instruction) Parse(content string) *Instruction {
	content = strings.TrimRight(content, ";")
	elements := strings.Split(content, ",")

	var args = make([]string, 0, len(elements))
	for _, e := range elements {
		ss := strings.SplitN(e, ".", 2)
		if len(ss) < 2 {
			continue
		}
		args = append(args, ss[1])
	}

	if len(args) == 0 {
		return NewInstruction("")
	}
	return NewInstruction(args[0], args[1:]...)
}
