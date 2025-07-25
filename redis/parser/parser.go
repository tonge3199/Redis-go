package parser

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/tonge3199/redis_go/interface/redis"
	"github.com/tonge3199/redis_go/redis/protocol"
	"io"
	"runtime/debug"
	"strconv"
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

// parse0 是Redis RESP协议解析的核心函数，处理TCP流中的RESP协议数据
//
// 协议解析流程说明：
// 1. 逐行读取：ReadBytes('\n')读取到\n为止的一行数据
// 2. 清理换行：bytes.TrimSuffix(line, []byte{'\r', '\n'})移除行尾的\r\n
// 3. 根据首字节决定协议类型，后续内容可能分布在多行
//
// RESP协议格式说明：
// + 简单字符串：首行包含完整内容 +OK\r\n
// - 错误：首行包含完整错误信息 -ERR unknown command\r\n
// : 整数：首行包含完整数值 :1000\r\n
// $ 批量字符串：$5\r\nhello\r\n
//	 首行声明长度（$5\r\n），reader继续读取下一行的实际内容（hello\r\n）
// * 数组：*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
//   首行声明元素数量（*2\r\n），reader继续读取每个元素的完整协议（$3\r\nfoo\r\n$3\r\nbar\r\n）
//
// 实际Redis交互示例：
//
// 示例1：SET命令响应流程
// 服务器响应：+OK\r\n
// 处理：case '+'读取首行，无需后续读取，直接返回StatusReply("OK")
//
// 示例2：GET命令响应流程
// 服务器响应：$5\r\nhello\r\n
// 处理：case '$'读取首行"$5"，parseBulkString继续读取下一行"hello\r\n"
//
// 示例3：LRANGE命令响应流程
// 服务器响应：*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n
// 处理：case '*'读取首行"*2"，parseArray继续读取：
//   - 第1个元素：$5\r\nhello\r\n
//   - 第2个元素：$5\r\nworld\r\n
//
// 示例4：FULLRESYNC复制同步
// 服务器响应：+FULLRESync repl_id offset\r\n$1024\r\n<binary_data>
// 处理：case '+'读取首行"+FULLRESYNC..."，检测到FULLRESYNC前缀，调用parseRDBBulkString继续读取RDB数据

func parse0(rawReader io.Reader, ch chan<- *Payload) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err, string(debug.Stack()))
		}
	}()
	reader := bufio.NewReader(rawReader)
	for {
		// 读取整行到\n为止，包含\r\n在内
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
			// 处理FULLRESYNC复制同步的特殊情况
			// 格式：+FULLRESYNC repl_id offset\r\n$length\r\n<rdb_binary_data>
			if strings.HasPrefix(content, "FULLRESYNC") {
				err = parseRDBBulkString(reader, ch)
				if err != nil {
					ch <- &Payload{Err: err}
					close(ch)
					return
				}
			}
		case '-':
			// 错误响应：首行即完整内容，格式：-error_message
			// 示例：-ERR unknown command -> ErrReply("ERR unknown command")
			ch <- &Payload{
				Data: protocol.MakeErrReply(string(line[1:])),
			}
		case ':':
			// 整数响应：首行即完整内容，格式：:number
			// 示例：:1000 -> IntReply(1000)
			value, err := strconv.ParseInt(string(line[1:]), 10, 64)
			if err != nil {
				protocolError(ch, "illegal number "+string(line[1:]))
				continue
			}
			ch <- &Payload{
				Data: protocol.MakeIntReply(value),
			}
		case '$':
			// 批量字符串：首行声明长度，reader继续读取下一行的实际内容
			// 格式：$length\r\n<content>\r\n
			// 示例：$5\r\nhello\r\n -> BulkReply("hello")
			err = parseBulkString(line, reader, ch)
			if err != nil {
				ch <- &Payload{Err: err}
				close(ch)
				return
			}
		case '*':
			// 数组：首行声明元素数量，reader继续读取每个元素的完整协议
			// 格式：*count\r\n<element1>\r\n<element2>\r\n...
			// 示例：*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n -> MultiBulkReply(["foo", "bar"])
			err = parseArray(line, reader, ch)
			if err != nil {
				ch <- &Payload{Err: err}
				close(ch)
				return
			}
		default:
			// 向后兼容：将空格分割的参数作为数组处理（非标准RESP格式）
			args := bytes.Split(line, []byte{' '})
			ch <- &Payload{
				Data: protocol.MakeMultiBulkReply(args),
			}
		}
	}
}

// parseArray:
// - 解析数组长度：nStrs = 2
// - 循环读取每个元素：
// - 第1次：读取 $3\r\nfoo\r\n → 解析出 "foo"
// - 第2次：读取 $3\r\nbar\r\n → 解析出 "bar"
// - 返回 MultiBulkReply(["foo", "bar"])
func parseArray(header []byte, reader *bufio.Reader, ch chan<- *Payload) error {
	nStrs, err := strconv.ParseInt(string(header[1:]), 10, 32)
	if err != nil || nStrs < 0 {
		protocolError(ch, "illegal array header "+string(header[1:]))
		return nil
	} else if nStrs == 0 {
		ch <- &Payload{
			Data: protocol.MakeEmptyMultiBulkReply(),
		}
		return nil
	}
	lines := make([][]byte, nStrs)
	for i := int64(0); i < nStrs; i++ {
		var line []byte
		line, err = reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		length := len(line)
		// length at least 4. eg: $1\r\n
		if length < 4 || line[length-2] != '\r' || line[0] != '$' {
			protocolError(ch, "illegal bulk string header "+string(line))
			break
		}
		// parse Bulk String logic again
		strLen, err := strconv.ParseInt(string(line[1:length-2]), 10, 64)
		if err != nil || strLen < -1 {
			protocolError(ch, "illegal bulk string header: "+string(line))
			break
		} else if strLen == -1 {
			lines = append(lines, []byte{})
		} else {
			body := make([]byte, strLen+2)
			_, err = io.ReadFull(reader, body)
			if err != nil {
				return err
			}
			lines = append(lines, body[:strLen]) // == body[:len(body)-2]
		}

	}
	ch <- &Payload{
		Data: protocol.MakeMultiBulkReply(lines),
	}
	return nil
}

// parseBulkString 解析RESP批量字符串协议
//
// 调用上下文：
// - 由parse0()在case '$'分支中调用
// - 传入参数header是parse0中已处理的当前行（已移除\r\n），格式如："$5"或"$-1"
// - reader是parse0中创建的bufio.Reader，用于继续读取后续内容
//
// RESP批量字符串协议格式：
// $length\r\ncontent\r\n
// 或：$-1\r\n（表示nil值）
//
// 实际调用流程示例：
//
// 示例1：正常字符串值
// parse0读取：$5\r\nhello\r\n
// 传入header：[]byte("$5")  // 已移除\r\n
// 计算strLen：5
// 读取内容：reader.ReadFull读取5+2=7字节（"hello\r\n"）
// 返回：BulkReply("hello")
//
// 示例2：空字符串
// parse0读取：$0\r\n\r\n
// 传入header：[]byte("$0")
// 计算strLen：0
// 读取内容：reader.ReadFull读取0+2=2字节（"\r\n"）
// 返回：BulkReply("")
//
// 示例3：nil值
// parse0读取：$-1\r\n
// 传入header：[]byte("$-1")
// 计算strLen：-1
// 直接返回：NullBulkReply()
//
// 示例4：二进制数据
// parse0读取：$1024\r\n<1024字节二进制数据>\r\n
// 传入header：[]byte("$1024")
// 计算strLen：1024
// 读取内容：reader.ReadFull读取1024+2=1026字节
// 返回：BulkReply(1024字节数据)
func parseBulkString(header []byte, reader *bufio.Reader, ch chan<- *Payload) error {
	// 输入: header = []byte("$5")
	// string(header[1:]) = "5"  (去掉 '$' 前缀)
	// strLen = 5

	// 输入: header = []byte("$-1")
	// string(header[1:]) = "-1"
	// strLen = -1
	strLen, err := strconv.ParseInt(string(header[1:]), 10, 64)
	if err != nil || strLen < -1 {
		// 错误情况：
		// "$abc"  -> 无法解析为数字
		// "$-2"   -> 长度小于-1（只允许-1表示null）
		protocolError(ch, "illegal bulk string header: "+string(header))
		return nil
	} else if strLen == -1 {
		// special null case.
		// 输入: "$-1\r\n"
		// 输出: NullBulkReply（表示 Redis 中的 nil 值）
		// 类似于其他语言中的 null 或 None
		ch <- &Payload{
			Data: protocol.MakeNullBulkReply(),
		}
		return nil
	}
	body := make([]byte, strLen+2)
	// ReadFull to optimize : must contain full(strLen+2) size: data+\r\n
	// strLen = 5
	// body = make([]byte, 5+2) = make([]byte, 7)
	// 读取内容: "hello\r\n" (7字节)
	_, err = io.ReadFull(reader, body)
	if err != nil {
		return err
	}
	ch <- &Payload{
		// body = []byte("hello\r\n")  // 7字节
		// len(body) = 7
		// body[:len(body)-2] = body[:5] = []byte("hello")  // 去掉末尾的\r\n
		Data: protocol.MakeBulkReply(body[:len(body)-2]),
	}
	return nil
}

// 这个函数专门用于解析 Redis 主从复制过程中的 RDB 文件传输。
//  1. 从节点连接主节点
//  2. 主节点发送: "+FULLRESYNC <runid> <offset>\r\n"
//  3. 主节点发送: "$<rdb_size>\r\n<rdb_binary_data><aof_data>..."
//     ^^^^^^^^^^^^^^^^^^^^^^^^
//     注意：RDB数据后面直接跟AOF数据，没有\r\n分隔
//
// there is no CRLF between RDB and following AOF, therefore it needs to be treated differently
func parseRDBBulkString(reader *bufio.Reader, ch chan<- *Payload) error {

	// 网络输入: "$2048576\r\n[RDB数据][AOF数据]..."
	// header = []byte("$2048576\r\n")
	header, err := reader.ReadBytes('\n')
	if err != nil {
		return errors.New("failed to read bytes")
	}
	header = bytes.TrimSuffix(header, []byte{'\r', '\n'})
	if len(header) == 0 {
		return errors.New("empty header")
	}
	strLen, err := strconv.ParseInt(string(header[1:]), 10, 64)
	if err != nil || strLen <= 0 {
		return errors.New("illegal bulk header" + string(header))
	}
	body := make([]byte, strLen) // pure RDB data
	_, err = io.ReadFull(reader, body)
	if err != nil {
		return err
	}
	ch <- &Payload{
		// body[:len(body)]
		Data: protocol.MakeBulkReply(body[:]),
	}
	return nil
}

func protocolError(ch chan<- *Payload, msg string) {
	err := errors.New("protocol error: " + msg)
	ch <- &Payload{Err: err}
}
