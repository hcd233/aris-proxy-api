# Model List 嵌入 Endpoint 详细信息

## 背景

当前 `GET /api/v1/model/list` 接口只返回 `endpointID`，前端需要额外调用 `GET /api/v1/endpoint/list` 来获取 endpoint 的详细信息（如名称）。这导致：
1. 前端需要两次 API 调用才能显示完整的 model 列表
2. 数据冗余：同一个 endpoint 信息可能被多个 model 引用

## 目标

1. 修改 model list 接口，返回嵌套的 endpoint 详细信息
2. 复用 `EndpointItem` 数据结构
3. 移除 `endpointID` 字段，改为嵌套 `endpoint` 对象
4. 前端模型列表页不再需要在页面加载时调用 list endpoint

## 设计

### 后端变更

#### 1. 更新 ModelView (`internal/application/model/query/list_models.go`)

```go
// ModelView Model 只读投影
type ModelView struct {
    ID        uint
    Alias     string
    ModelName string
    Endpoint  *EndpointView  // 嵌套 endpoint 详细信息
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### 2. 更新 listModelsHandler

- 添加 `endpointRepo llmproxy.EndpointRepository` 依赖
- 在 `Handle()` 中为每个 model 查询对应的 endpoint 详细信息
- 使用 `endpointRepo.FindByID()` 获取 endpoint

#### 3. 更新 ModelItem (`internal/dto/model.go`)

```go
// ModelItem Model 列表项
type ModelItem struct {
    ID        uint          `json:"id" doc:"Model ID"`
    Alias     string        `json:"alias" doc:"模型别名"`
    ModelName string        `json:"modelName" doc:"上游实际模型名"`
    Endpoint  *EndpointItem `json:"endpoint" doc:"关联 Endpoint 详细信息"`
    CreatedAt time.Time     `json:"createdAt" doc:"创建时间"`
    UpdatedAt time.Time     `json:"updatedAt" doc:"更新时间"`
}
```

#### 4. 更新 handler 映射

在 `modelHandler.HandleListModels()` 中，将 `ModelView.Endpoint` 映射为 `dto.EndpointItem`。

### 前端变更

#### 1. 更新 ModelItem 类型 (`web/src/lib/types.ts`)

```typescript
export interface ModelItem {
  id: number;
  alias: string;
  modelName: string;
  endpoint: EndpointItem;  // 嵌套 endpoint 详细信息
  createdAt: string;
  updatedAt: string;
}
```

#### 2. 更新 models/page.tsx

- 移除页面加载时的 `api.listEndpoints()` 调用
- 移除 `endpoints` state
- 添加 `fetchEndpoints()` 函数，仅在打开创建/编辑对话框时调用
- 更新 `openCreate()` 和 `openEdit()` 使用 `model.endpoint.id`
- 更新 `getEndpointName()` 使用 `model.endpoint.name`

### 性能考虑

- 每个 model 需要额外查询一次 endpoint，但 endpoint 数量通常较少
- 如果未来 model 数量很大，可以考虑：
  - 批量查询 endpoints（使用 IN 查询）
  - 在 model 聚合根中缓存 endpoint 信息

## 影响范围

- `internal/application/model/query/list_models.go`
- `internal/dto/model.go`
- `internal/handler/model.go`
- `internal/bootstrap/container.go`
- `web/src/lib/types.ts`
- `web/src/app/(dashboard)/models/page.tsx`

## 测试计划

1. 单元测试：更新 model repository 和 handler 测试
2. E2E 测试：验证 model list 返回嵌套的 endpoint 信息
3. 前端测试：验证模型列表页正确显示 endpoint 信息
