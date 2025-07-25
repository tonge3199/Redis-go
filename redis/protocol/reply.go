package protocol

import (
	"bytes"
	"errors"
	"github.com/tonge3199/redis_go/interface/redis"
	"strconv"
)

var (

	// CRLF is the line separator of redis serialization protocol
	CRLF = "\r\n"
)

/* ---- Bulk Reply ---- */

// BulkReply stores a binary-safe string
type BulkReply struct {
	Arg []byte
}

func MakeBulkReply(arg []byte) *BulkReply {
	return &BulkReply{Arg: arg}
}

/*
ToBytes 序列化BulkReply为Redis协议格式的字节流

示例1: 普通字符串"hello" → "$5\r\nhello\r\n"

示例2: 空字符串"" → "$0\r\n\r\n"

示例3: nil值 → "$-1\r\n" (Null Bulk)
*/
func (r *BulkReply) ToBytes() []byte {
	if r.Arg == nil {
		return nullBulkBytes
	}
	return []byte("$" + strconv.Itoa(len(r.Arg)) + CRLF + string(r.Arg) + CRLF)
}

/* ---- Multi Bulk Reply ---- */

// MultiBulkReply stores a list of string
type MultiBulkReply struct {
	Args [][]byte
}

func MakeMultiBulkReply(args [][]byte) *MultiBulkReply {
	return &MultiBulkReply{Args: args}
}

// ToBytes marshal redis.Reply
//
// 序列化MultiBulkReply为Redis协议格式的字节流
//
// 示例1: 空数组[] → "*0\r\n"
//
// 示例2: 包含两个元素["foo", "bar"] → "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
//
// 示例3: 包含nil元素["foo", nil, "bar"] → "*3\r\n$3\r\nfoo\r\n$-1\r\n$3\r\nbar\r\n"
func (r *MultiBulkReply) ToBytes() []byte {
	var buf bytes.Buffer
	// Calculate the length of buffer
	argLen := len(r.Args)
	bufLen := 1 + len(strconv.Itoa(argLen)) + 2 // "*" + count + CRLF
	for _, arg := range r.Args {
		if arg == nil {
			bufLen += 3 + 2 // "$-1" + CRLF
		} else {
			// Examples of len(strconv.Itoa(len(arg))):
			// - arg = []byte("hi")           -> len(arg) = 2   -> strconv.Itoa(2) = "2"     -> len("2") = 1 digit
			// - arg = []byte("hello world")  -> len(arg) = 11  -> strconv.Itoa(11) = "11"   -> len("11") = 2 digits
			// - arg = []byte(make([]byte, 256)) -> len(arg) = 256 -> strconv.Itoa(256) = "256" -> len("256") = 3 digits
			// - arg = []byte(make([]byte, 1024)) -> len(arg) = 1024 -> strconv.Itoa(1024) = "1024" -> len("1024") = 4 digits
			//
			// So len(strconv.Itoa(len(arg))) gives us the number of digits needed to represent the byte length:
			// - 1-9 bytes     -> 1 digit  (e.g., "5")
			// - 10-99 bytes   -> 2 digits (e.g., "53")
			// - 100-999 bytes -> 3 digits (e.g., "256")
			// - 1000+ bytes   -> 4+ digits (e.g., "1024")
			//
			// Complete example for arg = []byte("hello"):
			// len(arg) = 5
			// strconv.Itoa(5) = "5"
			// len("5") = 1
			// Formula: 1 + 1 + 2 + 5 + 2 = 11 bytes total
			// Result: "$5\r\nhello\r\n" (exactly 11 bytes)
			//
			//        $ + length string  + CRLF + data  + CRLF
			bufLen += 1 + len(strconv.Itoa(len(arg))) + 2 + len(arg) + 2
		}
	}
	// Allocate memory
	buf.Grow(bufLen)

	// Write string step by step, avoid concat strings
	// 逐个写入数组元素，避免字符串拼接带来的性能损耗
	//
	// 示例处理过程:
	// 1. 写入数组标识和长度: "*2\r\n"
	buf.WriteString("*")
	buf.WriteString(strconv.Itoa(argLen))
	buf.WriteString(CRLF)
	for _, arg := range r.Args {
		if arg == nil {
			//  "$-1\r\n
			buf.WriteString("$-1")
			buf.WriteString(CRLF)
		} else {
			//  "$3\r\n" , 3 can be any num
			buf.WriteString("$")
			buf.WriteString(strconv.Itoa(len(arg)))
			buf.WriteString(CRLF)
			// Write bytes, avoid slice of byte to string
			//
			// "$3\r\nbar\r\n"
			buf.Write(arg)
			buf.WriteString(CRLF)
		}
	}
	return buf.Bytes()
}

/* ---- Multi Raw Reply ---- */

// MultiRawReply store complex list structure, for example GeoPos command
//
// Example for GEOPOS command:
// Input: MultiRawReply{Replies: []redis.Reply{
//     &BulkReply{Arg: []byte("116.38888888888889")},  // longitude
//     &BulkReply{Arg: []byte("39.92888888888889")},   // latitude
//     &BulkReply{Arg: nil},                              // nil for missing member
//     &BulkReply{Arg: []byte("114.06638888888889")},   // longitude
//     &BulkReply{Arg: []byte("22.533888888888889")},   // latitude
// }}
// Output: "*5\r\n$17\r\n116.38888888888889\r\n$14\r\n39.92888888888889\r\n$-1\r\n$17\r\n114.06638888888889\r\n$15\r\n22.533888888888889\r\n"
//
// Example structure for GEOPOS with multiple coordinates:
// Each coordinate pair is represented as [longitude, latitude] in the array
// Missing coordinates are represented as nil BulkReply elements
// The entire response is wrapped as a MultiRawReply containing individual BulkReply elements

type MultiRawReply struct {
	Replies []redis.Reply
}

func MakeMultiRawReply(replies []redis.Reply) *MultiRawReply {
	return &MultiRawReply{Replies: replies}
}

// ToBytes marshal redis.Reply
//
// 用于序列化复杂列表结构，特别适用于GEOPOS等返回嵌套数组的命令
//
// GEOPOS命令示例:
//
// 输入: GEOPOS Sicily Palermo Catania
//
//	输出结构: MultiRawReply{Replies: []redis.Reply{
//	    &MultiBulkReply{Args: [][]byte{[]byte("13.36138933897018433"), []byte("38.11555639549629859")}}, // Palermo坐标
//	    &MultiBulkReply{Args: [][]byte{[]byte("15.08726745843887329"), []byte("37.50266842333162032")}}, // Catania坐标
//	}}
//
// 序列化输出: "*2\r\n*2\r\n$20\r\n13.36138933897018433\r\n$19\r\n38.11555639549629859\r\n*2\r\n$20\r\n15.08726745843887329\r\n$19\r\n37.50266842333162032\r\n"
//
// 包含nil值的示例:
//
// 输入: GEOPOS Sicily NonExisting Palermo
//
//	输出结构: MultiRawReply{Replies: []redis.Reply{
//	    &BulkReply{Arg: nil}, // 不存在的成员返回nil
//	    &MultiBulkReply{Args: [][]byte{[]byte("13.36138933897018433"), []byte("38.11555639549629859")}},
//	}}
//
// 序列化输出: "*2\r\n$-1\r\n*2\r\n$20\r\n13.36138933897018433\r\n$19\r\n38.11555639549629859\r\n"
func (r *MultiRawReply) ToBytes() []byte {
	argLen := len(r.Replies)
	var buf bytes.Buffer
	buf.WriteString("*" + strconv.Itoa(argLen) + CRLF)
	for _, reply := range r.Replies {
		buf.Write(reply.ToBytes())
	}
	return buf.Bytes()
}

/* ---- Status Reply ---- */

// StatusReply stores a simple status string
type StatusReply struct {
	Status string
}

// MakeStatusReply creates StatusReply
func MakeStatusReply(status string) *StatusReply {
	return &StatusReply{
		Status: status,
	}
}

// ToBytes marshal redis.Reply
func (r *StatusReply) ToBytes() []byte {
	return []byte("+" + r.Status + CRLF)
}

// IsOKReply returns true if the given protocol is +OK
func IsOKReply(reply redis.Reply) bool {
	return string(reply.ToBytes()) == "+OK\r\n"
}

/* ---- Int Reply ---- */

// IntReply stores an int64 number
type IntReply struct {
	Code int64
}

// MakeIntReply creates int protocol
func MakeIntReply(code int64) *IntReply {
	return &IntReply{
		Code: code,
	}
}

func (r *IntReply) ToBytes() []byte {
	return []byte(":" + strconv.FormatInt(r.Code, 10) + CRLF)
}

/* ---- Error Reply ---- */

// ErrorReply is an error and redis.Reply
type ErrorReply interface {
	Error() string
	ToBytes() []byte
}

// StandardErrReply represents server error
type StandardErrReply struct {
	Status string
}

// MakeErrReply creates StandardErrReply
func MakeErrReply(status string) *StandardErrReply {
	return &StandardErrReply{
		Status: status,
	}
}

// IsErrorReply returns true if the given protocol is error
func IsErrorReply(reply redis.Reply) bool {
	return reply.ToBytes()[0] == '-'
}

// Try2ErrorReply 将Redis协议回复转换为Go错误
// 用于从Redis响应中提取错误信息并转换为标准的Go error类型
//
// 示例1 - 错误回复:
// 输入: &StandardErrReply{Status: "ERR unknown command 'foo'"}
// 输出: error("ERR unknown command 'foo'")
// 协议格式: "-ERR unknown command 'foo'\r\n"
//
// 示例2 - 正常回复:
// 输入: &StatusReply{Status: "OK"}
// 输出: nil (因为不是错误回复)
// 协议格式: "+OK\r\n"
//
// 示例3 - 空回复:
// 输入: 空字符串或nil回复
// 输出: error("empty reply")
//
// 使用场景:
// 通常在客户端接收到Redis响应后，需要检查是否为错误时调用
// 例如: if err := Try2ErrorReply(reply); err != nil { return err }
func Try2ErrorReply(reply redis.Reply) error {
	str := string(reply.ToBytes())
	if len(str) == 0 {
		return errors.New("empty reply")
	}
	if str[0] != '-' {
		return nil
	}
	return errors.New(str[1:])
}

// ToBytes marshal redis.Reply
func (r *StandardErrReply) ToBytes() []byte {
	return []byte("-" + r.Status + CRLF)
}

func (r *StandardErrReply) Error() string {
	return r.Status
}
