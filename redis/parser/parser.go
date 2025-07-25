package parser

import (
	"bufio"
	"bytes"
	"github.com/tonge3199/redis_go/interface/redis"
	"github.com/tonge3199/redis_go/redis/protocol"
	"io"
	"runtime/debug"
	"strings"
)

type Payload struct {
	Data redis.Reply
	Err  error
}

// ParseStream reads data from io.Reader and send payloads through channel
func ParseStream(rd io.Reader) <-chan *Payload {
	ch := make(chan *Payload)
	go parse0(rd, ch)
	return ch
}

func parse0(rawReader io.Reader, ch chan<- *Payload) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err, string(debug.Stack()))
		}
	}()
	reader := bufio.NewReader(rawReader)
	for {
		// 逐行读取数据，直到遇到'\n'字符
		// 示例1: 读取状态回复 "+OK\r\n" → line = "+OK\r\n"
		// 示例2: 读取错误回复 "-ERR unknown command\r\n" → line = "-ERR unknown command\r\n"
		// 示例3: 读取整数回复 ":1000\r\n" → line = ":1000\r\n"
		// 示例4: 读取批量字符串 "$6\r\nfoobar\r\n" → line = "$6\r\n"
		// 示例5: 读取数组 "*2\r\n" → line = "*2\r\n"
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// 当读取遇到错误(如EOF)时，将错误信息通过通道发送给调用方
			// 然后关闭通道并退出循环
			ch <- &Payload{Err: err}
			close(ch)
			return
		}

		length := len(line)
		// 检查行格式是否合法(长度至少为3且倒数第二个字符必须是'\r')
		// 特殊情况处理：在主从复制流量中可能存在空行，此时选择跳过而不是报错
		// 示例: 处理空行 "\r\n" (length=2) 或 "\n" (length=1) 时会跳过
		if length <= 2 || line[length-2] != '\r' {
			continue
		}
		// 移除行尾的"\r\n"字符，得到纯净的数据
		// 示例: "$6\r\n" → "$6"
		line = bytes.TrimSuffix(line, []byte{'\r', '\n'})

		// 根据行首字符判断Redis协议类型并进行相应处理
		switch line[0] {
		case '+':
			// 处理状态回复 (Status Reply)
			// 示例: "+OK" → content = "OK"
			// 示例: "+PONG" → content = "PONG"
			content := string(line[1:])
			ch <- &Payload{
				Data: protocol.MakeStatusReply(content),
			}
			if strings.HasPrefix(content,"FULLRESYNC") {
				
			}

		case '-':

		case '$'
		}
	}
}
