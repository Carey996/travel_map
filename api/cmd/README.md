# `api/cmd` · 可执行入口

> 知识树：[项目根](../../README.md) › [`api`](../README.md) › **`cmd`**

Go 惯例：`cmd/<名称>/` 下放 `package main`，一个目录一个可执行文件。
本期只有一个：`server`。

| 目录 | 产物 |
|---|---|
| [`server/`](server/README.md) | `travel-blog-api` HTTP 服务 |

```powershell
go run ./cmd/server                        # 直接运行
go build -o bin/server.exe ./cmd/server    # 编译成单文件
```

`bin/` 已在 `.gitignore` 中忽略。

## 相关

- 上级：[`api/`](../README.md)
- 子目录：[`server/`](server/README.md)
