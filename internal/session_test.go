package async

import (
	"github.com/Pentahill/actionflow/internal/channel"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExampleNewSessionManagerWithOptions 展示如何使用 SessionManagerOptions 定制化配置
func ExampleNewSessionManagerWithOptions() {
	server := &AsyncServer{} // 假设已创建

	// 示例1: 使用默认配置（最简单的方式）
	sessionManager1 := NewSessionManager(server, 100)
	logx.Debugf("Created session manager with default config")

	// 示例2: 使用自定义 ActionStorage
	customStorage := channel.NewMemoryActionStorage()
	sessionManager2 := NewSessionManagerWithOptions(server, &SessionManagerOptions{
		BufferSize:    200,
		ActionStorage: customStorage,
	})
	logx.Debugf("Created session manager with custom ActionStorage")

	// 示例3: 使用 nil 配置（使用所有默认值）
	sessionManager3 := NewSessionManagerWithOptions(server, nil)
	logx.Debugf("Created session manager with nil options (all defaults)")

	// 示例4: 使用空配置（使用所有默认值）
	sessionManager4 := NewSessionManagerWithOptions(server, &SessionManagerOptions{})
	logx.Debugf("Created session manager with empty options (all defaults)")

	// 使用示例
	_ = sessionManager1
	_ = sessionManager2
	_ = sessionManager3
	_ = sessionManager4
}

// ExampleNewSessionManagerWithOptions_redis 展示如何在生产环境中使用 Redis 存储（示例注释）
func ExampleNewSessionManagerWithOptions_redis() {
	server := &AsyncServer{}
	// 假设有一个 Redis 实现的 ActionStorage（需要自己实现）
	// redisStorage := channel.NewRedisActionStorage(redisClient)
	// sessionManager := NewSessionManagerWithOptions(server, &SessionManagerOptions{
	// 	BufferSize: 200, ActionStorage: redisStorage,
	// })
	_ = server
}

// ExampleNewSessionManagerWithOptions_empty 展示使用空配置的方式
func ExampleNewSessionManagerWithOptions_empty() {
	server := &AsyncServer{} // 假设已创建

	// 使用配置方式
	customStorage := channel.NewMemoryActionStorage()
	sessionManager := NewSessionManagerWithOptions(server, &SessionManagerOptions{
		BufferSize:    100,
		ActionStorage: customStorage,
	})

	_ = sessionManager
}

// ExampleSessionManager_Replay 展示如何使用重放功能（获取已存在会话进行重放）
func ExampleSessionManager_Replay() {
	server := &AsyncServer{} // 假设已创建
	sessionManager := NewSessionManager(server, 100)
	sessionID := "session-123"

	// 获取或创建会话后，可通过 Replay 获取已存在的会话进行重放消费
	_ = sessionManager.GetOrCreate(sessionID)
	session, ok := sessionManager.Replay(sessionID)
	if ok {
		logx.Debugf("Replay session found, session_id=%s", session.SessionID)
	}
	_ = session
}
