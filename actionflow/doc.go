// Package actionflow 提供异步事件流服务器的公开 API。
// 应用层应通过本包引用，而非 internal。
//
// 示例：
//
//	import "github.com/Pentahill/actionflow/pkg/actionflow"
//
//	server := actionflow.NewAsyncServer(&actionflow.ServerOptional{
//	    UserRequestHandler: userRequestHandler,
//	    EventStream:        actionflow.EventStream(myEventStream),
//	    AgentOutputHandler: agentOutputHandler,
//	    GetSessionID:       func(req *http.Request) string { return req.URL.Query().Get("session_id") },
//	    AgentResultCallback: agentResultCallback,
//	})
//	handler := actionflow.NewSSEHandler(server)
//	http.HandleFunc("/", handler.ServeHTTP)
package actionflow
