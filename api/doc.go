// Package actionflow 提供异步事件流服务器的公开 API。
// 应用层通过 api 包引用，勿直接使用 internal。
//
// 示例：
//
//	import "github.com/Pentahill/actionflow/api"
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
