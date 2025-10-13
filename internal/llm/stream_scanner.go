// 定义 streamScanner 流式响应扫描器
package llm

import (
	"io"
	"strings"
	"unicode/utf8"
)

// StreamScanner 流式响应扫描器
type StreamScanner struct {
	reader      io.Reader
	buffer      []byte // 读取的数据
	currentLine string // 保存的当前行数据
	err         error  // 注：如果 `err` 字段在整个执行过程中未被赋值（即没有发生错误），最终返回的 `err` 值为 `nil`
}

func (s *StreamScanner) SetReader(reader io.Reader) {
	s.reader = reader
}

// Scan 扫描下一行
func (s *StreamScanner) Scan() bool {
	if s.err != nil {
		return false
	}

	// 读取数据
	buf := make([]byte, 1024)
	n, err := s.reader.Read(buf)     // 注：io.Reader 接口的 Read 方法是有状态的：每次调用会从上次读取结束的位置继续读取
	if err != nil && err != io.EOF { // 有错误发生 且 错误不是 EOF 则退出。这个条件保证了在遇到正常结束（EOF）时能继续处理剩余数据
		s.err = err
		return false
	}

	s.buffer = append(s.buffer, buf[:n]...)

	// 查找换行符
	for i, b := range s.buffer {
		if b == '\n' {
			s.currentLine = string(s.buffer[:i]) // 正确保存行内容
			s.buffer = s.buffer[i+1:]            // 移除已处理的数据（注：这一步将中间的 '\n' 符跳过了）
			return true
		}
	}

	return err != io.EOF
}

// Text 返回当前行的文本
func (s *StreamScanner) Text() string {
	line := s.currentLine

	// 1. 处理回车换行符
	line = strings.TrimSuffix(line, "\r")

	// 2. 处理首尾空白
	line = strings.TrimSpace(line)

	// 3. 确保 UTF-8 有效性
	if !utf8.ValidString(line) {
		line = strings.ToValidUTF8(line, "�")
	}

	return line
}

// Err 返回扫描过程中的错误
func (s *StreamScanner) Err() error {
	return s.err
}
