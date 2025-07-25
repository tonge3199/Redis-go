package protocol

import (
	"bytes"
	"github.com/tonge3199/redis_go/interface/redis"
)

// PongReply is +PONG
type PongReply struct{}

var pongBytes = []byte("+PONG\r\n")

// ToBytes marshal redis.Reply
func (r *PongReply) ToBytes() []byte {
	return pongBytes
}

// OKReply is +OK
type OKReply struct{}

var OKBytes = []byte("+OK\r\n")

// ToBytes marshal redis.Reply
func (r *OKReply) ToBytes() []byte {
	return OKBytes
}

var theOKReply = new(OKReply)

// MakeOKReply returns a ok protocol
func MakeOKReply() *OKReply {
	return theOKReply
}

var nullBulkBytes = []byte("$-1\r\n")

// NullBulkReply is empty string
type NullBulkReply struct{}

// ToBytes marshal redis.Reply
func (r *NullBulkReply) ToBytes() []byte {
	return nullBulkBytes
}

// MakeNullBulkReply creates a new NullBulkReply
func MakeNullBulkReply() *NullBulkReply {
	return &NullBulkReply{}
}

var emptyMultiBulkBytes = []byte("*0\r\n")

// EmptyMultiBulkReply is a empty list
type EmptyMultiBulkReply struct{}

// ToBytes marshal redis.Reply
func (r *EmptyMultiBulkReply) ToBytes() []byte {
	return emptyMultiBulkBytes
}

func MakeEmptyMultiBulkReply() *EmptyMultiBulkReply {
	return &EmptyMultiBulkReply{}
}

func IsEmptyMultiBulkReply(reply redis.Reply) bool {
	return bytes.Equal(reply.ToBytes(), emptyMultiBulkBytes)
}

// NoReply respond nothing, for commands like subscribe
type NoReply struct{}

var noBytes = []byte("")

// ToBytes marshal redis.Reply
func (r *NoReply) ToBytes() []byte {
	return noBytes
}

// QueuedReply is +QUEUED
type QueuedReply struct{}

var queuedBytes = []byte("+QUEUED\r\n")

// ToBytes marshal redis.Reply
func (r *QueuedReply) ToBytes() []byte {
	return queuedBytes
}

var theQueuedReply = new(QueuedReply)

func MakeQueuedReply() *QueuedReply {
	return theQueuedReply
}
