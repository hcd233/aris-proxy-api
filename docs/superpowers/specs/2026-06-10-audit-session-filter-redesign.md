# Audit 和 Session 列表过滤组件重新设计

## 设计概述

本文档描述了对 aris-proxy-api Web 端 Audit 和 Session 列表过滤组件的重新设计，旨在改善视觉体验、提升筛选效率，并提供现代化的交互方式。

## 设计目标

1. **改善视觉体验**：参考 claude.ai 的简洁、现代设计风格
2. **提高筛选效率**：支持筛选预设和实时筛选
3. **优化移动端体验**：响应式设计，折叠式布局
4. **保持功能完整性**：不丢失现有筛选功能

## 设计方案

### 1. 整体布局

采用**紧凑型工具栏式筛选**设计，将筛选条件整合到紧凑的工具栏中：

- **桌面端**：筛选条件以水平工具栏形式显示，每个条件使用下拉菜单或选择器
- **移动端**：采用折叠式布局，常用条件直接显示，其他条件放在"更多筛选"按钮中

### 2. 视觉设计

参考 claude.ai 的设计风格：

- **颜色方案**：使用柔和的背景色（#f8f9fa）和白色卡片，配合蓝色强调色（#3b82f6）
- **间距系统**：使用 8px 网格系统，保持一致的间距
- **圆角**：使用 6-8px 圆角，营造现代感
- **阴影**：使用轻微的阴影提升层次感
- **字体**：使用系统字体栈，保持清晰可读

### 3. 筛选组件设计

#### 3.1 时间范围选择器

- 保持现有的 TimeRangePicker 组件
- 支持预设时间范围（1h, 6h, 24h, 7d, 30d）
- 支持自定义时间范围

#### 3.2 下拉选择器

- 用户（User）
- 模型（Model）
- 状态（Status）
- 评分（Score）- 仅适用于 Session

#### 3.3 搜索框

- 支持关键词搜索
- 实时搜索（防抖延迟触发，300-500ms）

#### 3.4 筛选条件标签

- 显示当前激活的筛选条件
- 支持快速删除单个条件
- 支持清除所有条件

### 4. 交互功能

#### 4.1 筛选预设/书签

- **保存预设**：用户可以保存当前筛选条件组合为预设
- **加载预设**：从预设列表中快速切换筛选条件
- **预设管理**：支持编辑和删除预设
- **默认预设**：系统提供几个常用预设（如"今天错误"、"高延迟请求"）

#### 4.2 实时筛选

- **防抖延迟触发**：筛选条件变化后延迟 300-500ms 触发请求
- **视觉反馈**：筛选条件变化时显示加载状态
- **错误处理**：请求失败时显示错误信息

#### 4.3 标签化筛选条件

- 显示当前激活的筛选条件为标签
- 支持点击标签快速删除
- 支持清除所有条件

### 5. 移动端优化

采用**折叠式布局**：

- **常用条件直接显示**：时间范围、用户、搜索框
- **其他条件折叠**：模型、状态、评分放在"更多筛选"按钮中
- **触摸友好**：确保所有交互元素符合 44px 最小触摸目标
- **响应式布局**：根据屏幕宽度自动调整布局

### 6. 数据展示优化

#### 6.1 表格/列表视图

- 保持现有的表格和列表视图
- 优化数据展示方式，提高可读性
- 支持排序功能

#### 6.2 空状态设计

- 显示友好的空状态提示
- 提供引导操作（如清除筛选条件）

### 7. 性能优化

- **防抖处理**：避免频繁请求
- **缓存机制**：缓存筛选选项数据
- **懒加载**：分页加载数据
- **虚拟滚动**：支持大量数据的高性能展示（可选）

## 技术实现

### 1. 组件结构

```
src/components/
├── filters/
│   ├── filter-toolbar.tsx          # 筛选工具栏主组件
│   ├── filter-presets.tsx          # 筛选预设管理
│   ├── filter-tags.tsx             # 筛选条件标签
│   ├── mobile-filter.tsx           # 移动端筛选组件
│   └── filter-provider.tsx         # 筛选状态管理
├── ui/
│   ├── time-range-picker.tsx       # 时间范围选择器（保持现有）
│   ├── select.tsx                  # 下拉选择器（保持现有）
│   └── input.tsx                   # 输入框（保持现有）
```

### 2. 状态管理

使用 React Context 和 useReducer 管理筛选状态：

```typescript
interface FilterState {
  timeRange: TimeRangeKey;
  customStart: string;
  customEnd: string;
  user: string;
  model: string;
  status: string;
  score: string;
  keyword: string;
  presets: FilterPreset[];
  activePreset: string | null;
}

type FilterAction =
  | { type: 'SET_TIME_RANGE'; payload: TimeRangeKey }
  | { type: 'SET_USER'; payload: string }
  | { type: 'SET_MODEL'; payload: string }
  | { type: 'SET_STATUS'; payload: string }
  | { type: 'SET_SCORE'; payload: string }
  | { type: 'SET_KEYWORD'; payload: string }
  | { type: 'SAVE_PRESET'; payload: FilterPreset }
  | { type: 'LOAD_PRESET'; payload: string }
  | { type: 'CLEAR_FILTERS' };
```

### 3. 防抖实现

使用自定义 hook 实现防抖：

```typescript
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(timer);
    };
  }, [value, delay]);

  return debouncedValue;
}
```

### 4. 响应式设计

使用 Tailwind CSS 的响应式工具：

```tsx
<div className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
  {/* 移动端垂直堆叠，桌面端水平排列 */}
</div>
```

## 用户体验流程

### 1. 桌面端流程

1. 用户进入 Audit/Session 页面
2. 筛选工具栏显示在页面顶部
3. 用户选择筛选条件（时间范围、用户、模型等）
4. 筛选条件变化后，300-500ms 后自动触发请求
5. 筛选条件显示为标签，支持快速删除
6. 用户可以保存当前筛选条件为预设
7. 用户可以从预设列表中快速切换

### 2. 移动端流程

1. 用户进入 Audit/Session 页面
2. 常用筛选条件（时间范围、用户、搜索框）直接显示
3. 用户点击"更多筛选"按钮展开其他条件
4. 用户选择筛选条件
5. 筛选条件变化后，自动触发请求
6. 用户可以保存/加载预设

## 设计规范

### 1. 颜色规范

- **主色调**：#3b82f6（蓝色）
- **背景色**：#f8f9fa（浅灰）
- **卡片背景**：#ffffff（白色）
- **文字颜色**：
  - 主要文字：#111827
  - 次要文字：#6b7280
  - 占位符文字：#9ca3af
- **边框颜色**：#e5e7eb
- **强调色**：
  - 成功：#10b981
  - 警告：#f59e0b
  - 错误：#ef4444

### 2. 间距规范

- **基础单位**：4px
- **常用间距**：4px, 8px, 12px, 16px, 24px, 32px
- **组件间距**：16px
- **内边距**：12px, 16px

### 3. 圆角规范

- **小圆角**：4px
- **中圆角**：6px
- **大圆角**：8px
- **全圆角**：9999px

### 4. 字体规范

- **字体栈**：`-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`
- **字体大小**：
  - 小字体：11px, 12px
  - 正文：14px
  - 标题：16px, 18px, 24px
- **行高**：1.5

### 5. 阴影规范

- **轻微阴影**：`0 1px 2px 0 rgba(0, 0, 0, 0.05)`
- **标准阴影**：`0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1)`
- **强调阴影**：`0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1)`

## 实现计划

### 阶段一：基础组件重构

1. 重构筛选工具栏组件
2. 实现筛选状态管理
3. 实现防抖搜索
4. 优化移动端布局

### 阶段二：高级功能实现

1. 实现筛选预设功能
2. 实现标签化筛选条件
3. 优化视觉设计
4. 添加动画效果

### 阶段三：测试和优化

1. 单元测试
2. 集成测试
3. 性能优化
4. 用户反馈收集

## 验收标准

1. **视觉体验**：符合 claude.ai 的现代设计风格
2. **筛选效率**：支持筛选预设和实时筛选
3. **移动端体验**：响应式设计，折叠式布局
4. **性能**：筛选响应时间 < 500ms
5. **可访问性**：符合 WCAG 2.1 AA 标准
6. **兼容性**：支持主流浏览器（Chrome, Firefox, Safari, Edge）

## 参考资源

- claude.ai 设计风格
- Tailwind CSS 文档
- shadcn/ui 组件库
- React 官方文档