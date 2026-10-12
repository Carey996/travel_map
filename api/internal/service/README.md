# `api/internal/service` · 业务逻辑

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`service`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `place.go` | 照片自动归入地点、地点日期范围继承 | `AutoAssignRadiusM`, `NearestPlace()`, `AssignPhoto()` |

这个包**不依赖 Gin**，只接收 `context.Context` 与 `*gorm.DB`，因此可以独立测试。

## 自动聚类

`AutoAssignRadiusM = 1500`：同一个地点内的照片通常相距很近，
1.5 km 能覆盖一座小城中心或一个景区。

`AssignPhoto()` 的逻辑：

`F`
照片没有 GPS            -> 返回 nil，不归属任何地点（落进前端的「未定位」托盘）
照片已有 place_id       -> 取该地点，仅更新日期范围
1.5 km 内有已存在地点   -> 归入
否则                    -> 就地新建一个「待命名」地点（geocoded=false），由前端随后补名字
`F`

「就地新建」把命名成本从后端转移到了前端：上传时不做反向地理编码（那会拖慢响应、
且地名服务可能不可达），而是由前端发现 `geocoded=false` 的地点后再异步补名。

## 日期范围继承

`extendDateRange()` 让地点的 `date_from` / `date_to` 自动覆盖其照片的拍摄日期。
这样前端地点列表显示的是**真实行程日期**，而不是「今天创建」。
只扩不缩：已有范围比新照片更宽时不动。

## ⚠️ 一个必须知道的调用约定

`AssignPhoto()` **只修改内存里的 `photo.PlaceID`，不写库**。
调用方（handler）必须显式回写一次。

这不是设计疏漏，而是因为照片的插入发生在聚类之前——用它来避免一次多余的 UPDATE。
但**回写条件必须是无条件的**：早期版本写成「值不同才更新」，
而 `AssignPhoto` 恰好已经把它设成了同一个值，于是唯一需要写库的那次被跳过，
表现为照片在地图上永远挂不到地点下。这个 bug 已修复，注释留在 `handler/photo.go` 里。

## 相关

- 上级：[`internal/`](../README.md)
- 调用方：[`handler/`](../handler/README.md)
