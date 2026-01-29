package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	async "github.com/Pentahill/actionflow/internal"
	"github.com/Pentahill/actionflow/internal/protocol"

	"github.com/zeromicro/go-zero/core/logx"
)

// eventStream 创建 EventStream（示例实现）
// 返回一个通道，用于发送事件数据到客户端
func eventStream(ctx context.Context) (<-chan any, error) {
	ch := make(chan any, 10)
	go func() {
		defer close(ch)
		// 示例：发送一些测试数据
		for i := 0; i < 5; i++ {
			time.Sleep(5 * time.Second)
			select {
			case <-ctx.Done():
				return
			case ch <- map[string]interface{}{
				"id":      i + 1,
				"content": fmt.Sprintf("message %d", i+1),
			}:
			}
		}
	}()
	return ch, nil
}

// userRequestHandler 处理用户请求
func userRequestHandler(ctx context.Context, req *async.UserRequest) (any, error) {
	ss := req.GetSession()
	sessionID := ss.ID()
	
	// 发送事件到客户端
	_, err := ss.Send(ctx, map[string]interface{}{
		"event": "invest_start",
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to send event, session_id=%s, error=%v", sessionID, err)
	}

	// TODO: 处理请求，例如持久化数据到数据库
	logx.WithContext(ctx).Debugf("Received client request, persisting data to database, session_id=%s, payload=%+v", sessionID, req.GetPayload())
	return nil, nil
}

// agentOutputHandler 处理 Agent 的输出
func agentOutputHandler(ctx context.Context, payload interface{}) error {
	// 处理 Agent 的输出
	logx.Debugf("Agent output: %v", payload)
	// 可以发送到客户端、存储到数据库等
	return nil
}

// agentResultCallback 处理 Agent 的结果回调
func agentResultCallback(ctx context.Context, response *async.AgentResponse) error {
	// 处理 Agent 的结果
	logx.Debugf("Agent result: %v", response)

	ss := response.GetSession()
	
	// 发送 Agent 结果到客户端
	_, err := ss.Send(ctx, response.GetPayload())
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to send agent result, error=%v", err)
		return err
	}

	return nil
}

func main() {
	// 可选：开启 Debug 级别查看完整流水日志（handler 注册、分发、会话等）
	// logx.SetLevel(logx.DebugLevel)

	// 创建 AsyncServer
	server := async.NewAsyncServer(&async.ServerOptional{
		UserRequestHandler: userRequestHandler,
		EventStream:        protocol.EventStream(eventStream),
		AgentOutputHandler: agentOutputHandler,
		GetSessionID: func(req *http.Request) string {
			// 从 URL 查询参数中获取 session_id
			return req.URL.Query().Get("session_id")
		},
		AgentResultCallback: agentResultCallback,
	})

	// 创建 SSE Handler
	sseHandler := async.NewSSEHandler(server)
	
	// 设置 HTTP 路由
	http.HandleFunc("/", sseHandler.ServeHTTP)

	// 启动 HTTP 服务器
	addr := ":8080"
	log.Printf("Starting HTTP server on %s", addr)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	// 在 goroutine 中启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("HTTP server started successfully on %s", addr)
	log.Println("Server is ready to handle requests")
	log.Println("Press Ctrl+C to stop the server")

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
