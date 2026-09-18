# `web/src/stores` · Pinia 状态

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`stores`**

全局状态。组件从这里读数据、调动作，**不直接访问后端**（一次性查询除外）。

## 文件

| 文件 | 职责 | 关键成员 |
|---|---|---|
| `auth.ts` | 当前用户与登录态 | `user`, `isAuthenticated`, `checking`, `restore()`, `login()`, `register()`, `logout()` |
| `gallery.ts` | 相册主状态：地点、照片、笔记、选中项 | 见下表 |

### gallery 的成员

| 类别 | 成员 |
|---|---|
| 数据 | `places`, `photos`, `unplacedPhotos`, `notes` |
| 状态 | `loading`, `notesLoading`, `selectedPlaceId`, `selectedPhotoId` |
| 派生 | `selectedPlace`, `selectedPhoto`, `locatedPhotos`, `photosOfSelectedPlace` |
| 动作 | `refresh()`, `selectPlace()`, `fillMissingNames()`, `createPlace()`, `updatePlace()`, `deletePlace()`, `mergePlace()`, `assignUnplacedTo()`, `updatePhoto()`, `deletePhoto()`, `createNote()`, `updateNote()`, `deleteNote()`, `reset()` |

## 为什么是「全量拉取 + 派生」

数据量不大（个人旅行博客，几百到几千张照片），所以启动时一次性把地点与照片全部拉下来，
在内存里派生各种视图，而不是给每个视图单独发请求。收益：

- **组件变得很薄**——侧栏、地图、抽屉都从同一份 `places` 读，不存在数据不一致
- **写入后同步很简单**——任何写操作后调一次 `refresh()`，全应用同时更新
- **地图与侧栏天然一致**——它们看到的是同一个数组

代价是照片数量到万级时需要改成分页。届时的分界点很清楚：把 `refresh()` 换成带游标的增量拉取。

## 两个容易忽略的行为

**`auth.restore()` 把 401 当作正常状态。** 启动时调 `/auth/me`，未登录会返回 401，
这不是错误，只是在把 `checking` 置回 false 表示「确认过了，未登录」。

**`fillMissingNames()` 会记住失败过的地点。** 上传后新建的地点没有名字，
它会调反向地理编码补上。若地名服务不可达，该地点会被移出「已尝试」集合以便下次重试——
但不会在本次循环里死循环。

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`api/`](../api/README.md) · [`lib/`](../lib/README.md) · [`components/`](../components/README.md) · [`i18n/`](../i18n/README.md) · [`styles/`](../styles/README.md)
