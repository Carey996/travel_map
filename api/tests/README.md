# `api/tests` · 后端测试

> 知识树：[项目根](../../README.md) › [`api`](../README.md) › **`tests`**

## 现状

目前只有一个空的 `__init__.py`。**测试尚未编写**——这一点必须说清楚，不要误以为这里已经覆盖了。

本项目当前的验证方式是**对真实运行中的服务做端到端验证**（真实 Postgres / Redis / MinIO）：

| 已验证项 | 结果 |
|---|---|
| 鉴权：注册、登录、Cookie 属性、会话撤销、CSRF 拦截、幂等 | 16/16 通过 |
| 业务链路：预签名直传、元数据登记、自动聚类、笔记、删除 | 23/24 通过 |
| 上传完整性与聚类边界（受控坐标） | 5/5 通过 |

这些验证脚本覆盖了真实依赖，但没有沉淀成可重复执行的测试文件，这是欠账。

## 建议的补测方向

```
tests/
├── conftest.py            # 用 aiosqlite 或测试库建表；fakeredis；内存 StorageBackend
├── test_auth.py           # 密码哈希、会话创建/撤销、CSRF 中间件
├── test_places.py         # 聚类边界（1499m / 1501m）、日期范围继承、合并
├── test_photos.py         # 幂等短路、无 GPS 照片、删除时对象清理
└── test_storage.py        # 预签名 URL 生成（纯计算，无需真实对象存储）
```

`pyproject.toml` 里的 `[tool.pytest.ini_options]` 已配好 `asyncio_mode = "auto"`，
dev 依赖里也已有 `pytest-asyncio` 与 `aiosqlite`，可以直接开始写。

## 相关

- 上级：[`api/`](../README.md)
