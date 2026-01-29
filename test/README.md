# test 目录说明

本目录用于**集成测试、端到端测试及测试辅助代码**，与源码目录下的 `*_test.go` 分工如下：

## 测试分层

| 类型     | 位置                    | 说明 |
|----------|-------------------------|------|
| 单元测试 | 与源码同目录的 `*_test.go` | 测试单个包，可访问未导出 API，随 `go test ./internal/...` 运行 |
| 集成测试 | `test/integration/`     | 通过包对外 API 测试多组件串联，可在此目录运行 |
| 测试工具 | `test/testutil/`        | 供各测试复用的 fixture、mock、helper 等 |

## 目录结构

```
test/
├── README.md           # 本说明
├── integration/        # 集成测试
│   └── *_test.go
├── testutil/           # 测试工具与共享 fixture（后续扩展）
│   └── ...
└── e2e/                # 端到端测试（可选，后续扩展）
    └── ...
```

## 运行方式

- 仅单元测试（不含集成）：`go test -short ./...`
- 含集成测试：`go test ./...` 或 `go test ./test/integration/...`
- 仅集成测试：`go test ./test/integration/...`

集成测试若较耗时，可用 `-short` 跳过（测试内已判断 `testing.Short()`）。
