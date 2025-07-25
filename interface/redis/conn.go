package redis

// Connection represents a connection with redis client
//
// # Connection 表示与 Redis 客户端的连接接口
//
// 该接口定义了与 Redis 服务器交互所需的所有基本操作
type Connection interface {
	// Write writes data to the connection and returns the number of bytes written and any error
	//
	// Write 向连接写入数据，返回写入的字节数和可能的错误
	//
	// 参数: data []byte - 要写入的字节数据
	//
	// 返回: int - 写入的字节数, error - 可能的错误
	Write([]byte) (int, error)

	// Close closes the connection
	//
	// Close 关闭连接
	//
	// 返回: error - 关闭连接时可能发生的错误
	Close() error

	// RemoteAddr returns the remote network address of the connection
	//
	// RemoteAddr 返回连接的远程网络地址
	//
	// 返回: string - 远程地址字符串 (例如: "127.0.0.1:6379")
	RemoteAddr() string

	// Authentication methods / 认证相关方法

	// SetPassword sets the password for Redis authentication
	//
	// SetPassword 设置 Redis 认证密码
	//
	// 参数: password string - Redis 服务器的认证密码
	SetPassword(string)

	// GetPassword returns the current password used for authentication
	//
	// GetPassword 返回当前用于认证的密码
	//
	// 返回: string - 当前设置的密码
	GetPassword() string

	// Pub/Sub methods / 发布订阅相关方法
	//
	// Client should keep its subscribing channels
	//
	// 客户端应该保持其订阅的频道列表

	// Subscribe adds a channel to the subscription list
	//
	// Subscribe 将频道添加到订阅列表
	//
	// 参数: channel string - 要订阅的频道名称
	Subscribe(channel string)

	// UnSubscribe removes a channel from the subscription list
	//
	// UnSubscribe 从订阅列表中移除频道
	//
	// 参数: channel string - 要取消订阅的频道名称
	UnSubscribe(channel string)

	// SubsCount returns the number of subscribed channels
	//
	// SubsCount 返回已订阅频道的数量
	//
	// 返回: int - 订阅频道的总数
	SubsCount() int

	// GetChannels returns all subscribed channels
	//
	// GetChannels 返回所有已订阅的频道
	//
	// 返回: []string - 包含所有订阅频道名称的切片
	GetChannels() []string

	// Transaction methods / 事务相关方法

	// InMultiState checks if the connection is in MULTI transaction state
	//
	// InMultiState 检查连接是否处于 MULTI 事务状态
	//
	// 返回: bool - true 表示在事务状态中，false 表示不在
	InMultiState() bool

	// SetMultiState sets the MULTI transaction state
	//
	// SetMultiState 设置 MULTI 事务状态
	//
	// 参数: state bool - true 开启事务状态，false 关闭事务状态
	SetMultiState(bool)

	// GetQueuedCmdLine returns all queued commands in the current transaction
	//
	// GetQueuedCmdLine 返回当前事务中所有排队的命令
	//
	// 返回: [][][]byte - 三维字节切片，每个命令由多个参数组成
	GetQueuedCmdLine() [][][]byte

	// EnqueueCmd adds a command to the transaction queue
	//
	// EnqueueCmd 将命令添加到事务队列中
	//
	// 参数: cmd [][]byte - 命令及其参数的字节切片数组
	EnqueueCmd([][]byte)

	// ClearQueuedCmds clears all queued commands in the transaction
	//
	// ClearQueuedCmds 清空事务中所有排队的命令
	ClearQueuedCmds()

	// GetWatching returns all keys being watched for changes
	//
	// GetWatching 返回所有被监视变化的键
	//
	// return: map[string]uint32 - 键名到版本号的映射
	GetWatching() map[string]uint32

	// AddTxError adds an error to the transaction error list
	//
	// AddTxError 向事务错误列表添加错误
	//
	// 参数: err error - 要添加的错误
	AddTxError(err error)

	// GetTxErrors returns all errors that occurred during the transaction
	//
	// GetTxErrors 返回事务期间发生的所有错误
	//
	// 返回: []error - 错误列表
	GetTxErrors() []error

	// Database selection methods / 数据库选择相关方法

	// GetDBIndex returns the current database index
	//
	// GetDBIndex 返回当前数据库索引
	//
	// 返回: int - 当前选择的数据库编号 (0-15)
	GetDBIndex() int

	// SelectDB selects a database by index
	//
	// SelectDB 通过索引选择数据库
	//
	// 参数: dbIndex int - 数据库索引 (通常为 0-15)
	SelectDB(int)

	// Replication role methods / 复制角色相关方法

	// SetSlave marks this connection as a slave
	//
	// SetSlave 将此连接标记为从节点
	SetSlave()

	// IsSlave checks if this connection is a slave
	//
	// IsSlave 检查此连接是否为从节点
	//
	// 返回: bool - true 表示是从节点，false 表示不是
	IsSlave() bool

	// SetMaster marks this connection as a master
	//
	// SetMaster 将此连接标记为主节点
	SetMaster()

	// IsMaster checks if this connection is a master
	//
	// IsMaster 检查此连接是否为主节点
	//
	// 返回: bool - true 表示是主节点，false 表示不是
	IsMaster() bool

	// Name returns the connection name/identifier
	//
	// Name 返回连接的名称或标识符
	//
	// 返回: string - 连接的唯一标识符
	Name() string
}
