# Audit 和 Session 列表过滤组件重新设计实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重新设计 aris-proxy-api Web 端 Audit 和 Session 列表过滤组件，改善视觉体验、提升筛选效率，并提供现代化的交互方式。

**Architecture:** 采用紧凑型工具栏式筛选设计，参考 claude.ai 的现代设计风格。使用 React Context 和 useReducer 管理筛选状态，实现防抖延迟触发和筛选预设功能。移动端采用折叠式布局优化。

**Tech Stack:** React, TypeScript, Tailwind CSS, shadcn/ui, Sonner (toast)

---

## 文件结构

```
web/src/
├── components/
│   ├── filters/
│   │   ├── filter-toolbar.tsx          # 筛选工具栏主组件
│   │   ├── filter-presets.tsx          # 筛选预设管理
│   │   ├── filter-tags.tsx             # 筛选条件标签
│   │   ├── mobile-filter.tsx           # 移动端筛选组件
│   │   └── filter-provider.tsx         # 筛选状态管理
│   ├── ui/
│   │   ├── time-range-picker.tsx       # 时间范围选择器（保持现有）
│   │   ├── select.tsx                  # 下拉选择器（保持现有）
│   │   └── input.tsx                   # 输入框（保持现有）
├── hooks/
│   ├── use-debounce.ts                 # 防抖 hook
│   └── use-filter-presets.ts           # 筛选预设 hook
├── lib/
│   ├── filter-context.tsx              # 筛选状态 Context
│   └── types.ts                        # 类型定义（更新）
├── app/
│   ├── (dashboard)/
│   │   ├── audit/
│   │   │   └── page.tsx                # Audit 页面（重构）
│   │   └── sessions/
│   │       └── page.tsx                # Sessions 页面（重构）
```

## 实现任务

### Task 1: 创建筛选状态管理和类型定义

**Files:**
- Create: `web/src/lib/filter-context.tsx`
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: 更新类型定义**

在 `web/src/lib/types.ts` 中添加筛选相关的类型定义：

```typescript
// 筛选状态类型
export interface FilterState {
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

// 筛选预设类型
export interface FilterPreset {
  id: string;
  name: string;
  filters: Omit<FilterState, 'presets' | 'activePreset'>;
  createdAt: string;
}

// 筛选动作类型
export type FilterAction =
  | { type: 'SET_TIME_RANGE'; payload: TimeRangeKey }
  | { type: 'SET_CUSTOM_START'; payload: string }
  | { type: 'SET_CUSTOM_END'; payload: string }
  | { type: 'SET_USER'; payload: string }
  | { type: 'SET_MODEL'; payload: string }
  | { type: 'SET_STATUS'; payload: string }
  | { type: 'SET_SCORE'; payload: string }
  | { type: 'SET_KEYWORD'; payload: string }
  | { type: 'SAVE_PRESET'; payload: FilterPreset }
  | { type: 'LOAD_PRESET'; payload: string }
  | { type: 'DELETE_PRESET'; payload: string }
  | { type: 'CLEAR_FILTERS' };
```

- [ ] **Step 2: 创建筛选状态 Context**

创建 `web/src/lib/filter-context.tsx`：

```typescript
"use client";

import { createContext, useContext, useReducer, type ReactNode } from "react";
import type { FilterState, FilterAction, FilterPreset } from "./types";
import type { TimeRangeKey } from "./time-range";

const initialState: FilterState = {
  timeRange: "24h",
  customStart: "",
  customEnd: "",
  user: "",
  model: "",
  status: "",
  score: "",
  keyword: "",
  presets: [],
  activePreset: null,
};

function filterReducer(state: FilterState, action: FilterAction): FilterState {
  switch (action.type) {
    case 'SET_TIME_RANGE':
      return { ...state, timeRange: action.payload, activePreset: null };
    case 'SET_CUSTOM_START':
      return { ...state, customStart: action.payload, activePreset: null };
    case 'SET_CUSTOM_END':
      return { ...state, customEnd: action.payload, activePreset: null };
    case 'SET_USER':
      return { ...state, user: action.payload, activePreset: null };
    case 'SET_MODEL':
      return { ...state, model: action.payload, activePreset: null };
    case 'SET_STATUS':
      return { ...state, status: action.payload, activePreset: null };
    case 'SET_SCORE':
      return { ...state, score: action.payload, activePreset: null };
    case 'SET_KEYWORD':
      return { ...state, keyword: action.payload, activePreset: null };
    case 'SAVE_PRESET':
      return {
        ...state,
        presets: [...state.presets, action.payload],
        activePreset: action.payload.id,
      };
    case 'LOAD_PRESET': {
      const preset = state.presets.find(p => p.id === action.payload);
      if (!preset) return state;
      return {
        ...state,
        ...preset.filters,
        activePreset: preset.id,
      };
    }
    case 'DELETE_PRESET':
      return {
        ...state,
        presets: state.presets.filter(p => p.id !== action.payload),
        activePreset: state.activePreset === action.payload ? null : state.activePreset,
      };
    case 'CLEAR_FILTERS':
      return {
        ...state,
        timeRange: "24h",
        customStart: "",
        customEnd: "",
        user: "",
        model: "",
        status: "",
        score: "",
        keyword: "",
        activePreset: null,
      };
    default:
      return state;
  }
}

interface FilterContextType {
  state: FilterState;
  dispatch: React.Dispatch<FilterAction>;
}

const FilterContext = createContext<FilterContextType | null>(null);

export function FilterProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(filterReducer, initialState);

  return (
    <FilterContext.Provider value={{ state, dispatch }}>
      {children}
    </FilterContext.Provider>
  );
}

export function useFilter() {
  const context = useContext(FilterContext);
  if (!context) {
    throw new Error('useFilter must be used within a FilterProvider');
  }
  return context;
}
```

- [ ] **Step 3: 创建防抖 hook**

创建 `web/src/hooks/use-debounce.ts`：

```typescript
"use client";

import { useState, useEffect } from "react";

export function useDebounce<T>(value: T, delay: number): T {
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

- [ ] **Step 4: 提交代码**

```bash
git add web/src/lib/filter-context.tsx web/src/lib/types.ts web/src/hooks/use-debounce.ts
git commit -m "feat: add filter state management and debounce hook"
```

### Task 2: 创建筛选工具栏主组件

**Files:**
- Create: `web/src/components/filters/filter-toolbar.tsx`

- [ ] **Step 1: 创建筛选工具栏组件**

创建 `web/src/components/filters/filter-toolbar.tsx`：

```typescript
"use client";

import { useFilter } from "@/lib/filter-context";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search, X } from "lucide-react";
import { useDebounce } from "@/hooks/use-debounce";
import { useEffect, useState } from "react";
import { FilterTags } from "./filter-tags";
import { FilterPresets } from "./filter-presets";
import { MobileFilter } from "./mobile-filter";
import { useIsMobile } from "@/hooks/use-mobile";

interface FilterToolbarProps {
  userOptions: string[];
  modelOptions: string[];
  statusOptions: string[];
  showScore?: boolean;
  onFilterChange: () => void;
}

export function FilterToolbar({
  userOptions,
  modelOptions,
  statusOptions,
  showScore = false,
  onFilterChange,
}: FilterToolbarProps) {
  const { state, dispatch } = useFilter();
  const isMobile = useIsMobile();
  const [keywordInput, setKeywordInput] = useState(state.keyword);
  const debouncedKeyword = useDebounce(keywordInput, 400);

  useEffect(() => {
    if (debouncedKeyword !== state.keyword) {
      dispatch({ type: 'SET_KEYWORD', payload: debouncedKeyword });
      onFilterChange();
    }
  }, [debouncedKeyword, state.keyword, dispatch, onFilterChange]);

  const handleTimeRangeChange = (key: string, cs: string, ce: string) => {
    dispatch({ type: 'SET_TIME_RANGE', payload: key as TimeRangeKey });
    dispatch({ type: 'SET_CUSTOM_START', payload: cs });
    dispatch({ type: 'SET_CUSTOM_END', payload: ce });
    onFilterChange();
  };

  const handleSelectChange = (type: string, value: string) => {
    const val = value === "__all__" ? "" : value;
    switch (type) {
      case 'user':
        dispatch({ type: 'SET_USER', payload: val });
        break;
      case 'model':
        dispatch({ type: 'SET_MODEL', payload: val });
        break;
      case 'status':
        dispatch({ type: 'SET_STATUS', payload: val });
        break;
      case 'score':
        dispatch({ type: 'SET_SCORE', payload: val });
        break;
    }
    onFilterChange();
  };

  const handleClearFilters = () => {
    dispatch({ type: 'CLEAR_FILTERS' });
    setKeywordInput("");
    onFilterChange();
  };

  const hasActiveFilters = state.user || state.model || state.status || state.score || state.keyword;

  if (isMobile) {
    return (
      <MobileFilter
        userOptions={userOptions}
        modelOptions={modelOptions}
        statusOptions={statusOptions}
        showScore={showScore}
        onFilterChange={onFilterChange}
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <TimeRangePicker
          value={state.timeRange}
          customStart={state.customStart}
          customEnd={state.customEnd}
          onChange={handleTimeRangeChange}
        />
        
        <Select
          value={state.user || "__all__"}
          onValueChange={(v) => handleSelectChange('user', v)}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="User" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All Users</SelectItem>
            {userOptions.map((u) => (
              <SelectItem key={u} value={u}>{u}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={state.model || "__all__"}
          onValueChange={(v) => handleSelectChange('model', v)}
        >
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="Model" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All Models</SelectItem>
            {modelOptions.map((m) => (
              <SelectItem key={m} value={m}>{m}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={state.status || "__all__"}
          onValueChange={(v) => handleSelectChange('status', v)}
        >
          <SelectTrigger className="w-[130px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All Status</SelectItem>
            {statusOptions.map((code) => (
              <SelectItem key={code} value={code}>{code}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        {showScore && (
          <Select
            value={state.score || "__all__"}
            onValueChange={(v) => handleSelectChange('score', v)}
          >
            <SelectTrigger className="w-[140px]">
              <SelectValue placeholder="Score" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All Scores</SelectItem>
              <SelectItem value="1">1</SelectItem>
              <SelectItem value="2">2</SelectItem>
              <SelectItem value="3">3</SelectItem>
              <SelectItem value="4">4</SelectItem>
              <SelectItem value="5">5</SelectItem>
              <SelectItem value="unscored">Unscored</SelectItem>
            </SelectContent>
          </Select>
        )}

        <div className="relative flex-1 md:w-64">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by traceID or model..."
            value={keywordInput}
            onChange={(e) => setKeywordInput(e.target.value)}
            className="pl-8"
          />
          {keywordInput && (
            <button
              type="button"
              onClick={() => {
                setKeywordInput("");
                dispatch({ type: 'SET_KEYWORD', payload: "" });
                onFilterChange();
              }}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          )}
        </div>

        {hasActiveFilters && (
          <Button
            variant="ghost"
            size="sm"
            className="gap-1 text-muted-foreground"
            onClick={handleClearFilters}
          >
            <X className="size-3.5" />
            Clear filters
          </Button>
        )}

        <FilterPresets />
      </div>

      <FilterTags onFilterChange={onFilterChange} />
    </div>
  );
}
```

- [ ] **Step 2: 提交代码**

```bash
git add web/src/components/filters/filter-toolbar.tsx
git commit -m "feat: add filter toolbar component"
```

### Task 3: 创建筛选条件标签组件

**Files:**
- Create: `web/src/components/filters/filter-tags.tsx`

- [ ] **Step 1: 创建筛选条件标签组件**

创建 `web/src/components/filters/filter-tags.tsx`：

```typescript
"use client";

import { useFilter } from "@/lib/filter-context";
import { Badge } from "@/components/ui/badge";
import { X } from "lucide-react";

interface FilterTagsProps {
  onFilterChange: () => void;
}

export function FilterTags({ onFilterChange }: FilterTagsProps) {
  const { state, dispatch } = useFilter();

  const tags: { key: string; label: string; value: string }[] = [];

  if (state.user) {
    tags.push({ key: 'user', label: 'User', value: state.user });
  }
  if (state.model) {
    tags.push({ key: 'model', label: 'Model', value: state.model });
  }
  if (state.status) {
    tags.push({ key: 'status', label: 'Status', value: state.status });
  }
  if (state.score) {
    tags.push({ key: 'score', label: 'Score', value: state.score });
  }
  if (state.keyword) {
    tags.push({ key: 'keyword', label: 'Search', value: state.keyword });
  }

  if (tags.length === 0) {
    return null;
  }

  const handleRemoveTag = (key: string) => {
    switch (key) {
      case 'user':
        dispatch({ type: 'SET_USER', payload: '' });
        break;
      case 'model':
        dispatch({ type: 'SET_MODEL', payload: '' });
        break;
      case 'status':
        dispatch({ type: 'SET_STATUS', payload: '' });
        break;
      case 'score':
        dispatch({ type: 'SET_SCORE', payload: '' });
        break;
      case 'keyword':
        dispatch({ type: 'SET_KEYWORD', payload: '' });
        break;
    }
    onFilterChange();
  };

  return (
    <div className="flex flex-wrap gap-2">
      {tags.map((tag) => (
        <Badge
          key={tag.key}
          variant="secondary"
          className="gap-1 pr-1.5"
        >
          <span className="text-xs">{tag.label}:</span>
          <span className="text-xs font-medium">{tag.value}</span>
          <button
            type="button"
            onClick={() => handleRemoveTag(tag.key)}
            className="ml-1 rounded-full p-0.5 hover:bg-muted-foreground/20"
          >
            <X className="size-3" />
          </button>
        </Badge>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: 提交代码**

```bash
git add web/src/components/filters/filter-tags.tsx
git commit -m "feat: add filter tags component"
```

### Task 4: 创建筛选预设管理组件

**Files:**
- Create: `web/src/components/filters/filter-presets.tsx`
- Create: `web/src/hooks/use-filter-presets.ts`

- [ ] **Step 1: 创建筛选预设 hook**

创建 `web/src/hooks/use-filter-presets.ts`：

```typescript
"use client";

import { useState, useEffect } from "react";
import type { FilterPreset } from "@/lib/types";

const STORAGE_KEY = "filter-presets";

export function useFilterPresets() {
  const [presets, setPresets] = useState<FilterPreset[]>([]);

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        setPresets(JSON.parse(stored));
      } catch {
        console.error("Failed to parse filter presets");
      }
    }
  }, []);

  const savePresets = (newPresets: FilterPreset[]) => {
    setPresets(newPresets);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newPresets));
  };

  const addPreset = (preset: FilterPreset) => {
    savePresets([...presets, preset]);
  };

  const deletePreset = (id: string) => {
    savePresets(presets.filter(p => p.id !== id));
  };

  return { presets, addPreset, deletePreset };
}
```

- [ ] **Step 2: 创建筛选预设组件**

创建 `web/src/components/filters/filter-presets.tsx`：

```typescript
"use client";

import { useState } from "react";
import { useFilter } from "@/lib/filter-context";
import { useFilterPresets } from "@/hooks/use-filter-presets";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Bookmark, Check, Trash2 } from "lucide-react";
import { toast } from "sonner";
import type { FilterPreset } from "@/lib/types";

export function FilterPresets() {
  const { state, dispatch } = useFilter();
  const { presets, addPreset, deletePreset } = useFilterPresets();
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [presetName, setPresetName] = useState("");

  const handleSavePreset = () => {
    if (!presetName.trim()) {
      toast.error("Please enter a preset name");
      return;
    }

    const preset: FilterPreset = {
      id: Date.now().toString(),
      name: presetName.trim(),
      filters: {
        timeRange: state.timeRange,
        customStart: state.customStart,
        customEnd: state.customEnd,
        user: state.user,
        model: state.model,
        status: state.status,
        score: state.score,
        keyword: state.keyword,
      },
      createdAt: new Date().toISOString(),
    };

    addPreset(preset);
    dispatch({ type: 'SAVE_PRESET', payload: preset });
    setSaveDialogOpen(false);
    setPresetName("");
    toast.success("Preset saved");
  };

  const handleLoadPreset = (preset: FilterPreset) => {
    dispatch({ type: 'LOAD_PRESET', payload: preset.id });
    toast.success(`Loaded preset: ${preset.name}`);
  };

  const handleDeletePreset = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    deletePreset(id);
    dispatch({ type: 'DELETE_PRESET', payload: id });
    toast.success("Preset deleted");
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="outline" size="sm" className="gap-1.5" />
          }
        >
          <Bookmark className="size-3.5" />
          Presets
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem onClick={() => setSaveDialogOpen(true)}>
            Save current filters
          </DropdownMenuItem>
          {presets.length > 0 && <DropdownMenuSeparator />}
          {presets.map((preset) => (
            <DropdownMenuItem
              key={preset.id}
              onClick={() => handleLoadPreset(preset)}
              className="flex items-center justify-between"
            >
              <div className="flex items-center gap-2">
                {state.activePreset === preset.id && (
                  <Check className="size-4" />
                )}
                <span>{preset.name}</span>
              </div>
              <button
                type="button"
                onClick={(e) => handleDeletePreset(e, preset.id)}
                className="rounded p-1 hover:bg-muted-foreground/20"
              >
                <Trash2 className="size-3" />
              </button>
            </DropdownMenuItem>
          ))}
          {presets.length === 0 && (
            <DropdownMenuItem disabled>
              No presets saved
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={saveDialogOpen} onOpenChange={setSaveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Save Filter Preset</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="preset-name">Preset Name</Label>
              <Input
                id="preset-name"
                value={presetName}
                onChange={(e) => setPresetName(e.target.value)}
                placeholder="Enter preset name"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSaveDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSavePreset}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
```

- [ ] **Step 3: 提交代码**

```bash
git add web/src/hooks/use-filter-presets.ts web/src/components/filters/filter-presets.tsx
git commit -m "feat: add filter presets component"
```

### Task 5: 创建移动端筛选组件

**Files:**
- Create: `web/src/components/filters/mobile-filter.tsx`

- [ ] **Step 1: 创建移动端筛选组件**

创建 `web/src/components/filters/mobile-filter.tsx`：

```typescript
"use client";

import { useState } from "react";
import { useFilter } from "@/lib/filter-context";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search, X, ChevronDown, ChevronUp } from "lucide-react";
import { FilterTags } from "./filter-tags";
import { FilterPresets } from "./filter-presets";
import type { TimeRangeKey } from "@/lib/time-range";

interface MobileFilterProps {
  userOptions: string[];
  modelOptions: string[];
  statusOptions: string[];
  showScore?: boolean;
  onFilterChange: () => void;
}

export function MobileFilter({
  userOptions,
  modelOptions,
  statusOptions,
  showScore = false,
  onFilterChange,
}: MobileFilterProps) {
  const { state, dispatch } = useFilter();
  const [expanded, setExpanded] = useState(false);
  const [keywordInput, setKeywordInput] = useState(state.keyword);

  const handleTimeRangeChange = (key: string, cs: string, ce: string) => {
    dispatch({ type: 'SET_TIME_RANGE', payload: key as TimeRangeKey });
    dispatch({ type: 'SET_CUSTOM_START', payload: cs });
    dispatch({ type: 'SET_CUSTOM_END', payload: ce });
    onFilterChange();
  };

  const handleSelectChange = (type: string, value: string) => {
    const val = value === "__all__" ? "" : value;
    switch (type) {
      case 'user':
        dispatch({ type: 'SET_USER', payload: val });
        break;
      case 'model':
        dispatch({ type: 'SET_MODEL', payload: val });
        break;
      case 'status':
        dispatch({ type: 'SET_STATUS', payload: val });
        break;
      case 'score':
        dispatch({ type: 'SET_SCORE', payload: val });
        break;
    }
    onFilterChange();
  };

  const handleSearch = () => {
    dispatch({ type: 'SET_KEYWORD', payload: keywordInput });
    onFilterChange();
  };

  const handleClearFilters = () => {
    dispatch({ type: 'CLEAR_FILTERS' });
    setKeywordInput("");
    onFilterChange();
  };

  const hasActiveFilters = state.user || state.model || state.status || state.score || state.keyword;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <TimeRangePicker
          value={state.timeRange}
          customStart={state.customStart}
          customEnd={state.customEnd}
          onChange={handleTimeRangeChange}
        />
        
        <Select
          value={state.user || "__all__"}
          onValueChange={(v) => handleSelectChange('user', v)}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="User" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All Users</SelectItem>
            {userOptions.map((u) => (
              <SelectItem key={u} value={u}>{u}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button
          variant="outline"
          size="sm"
          onClick={() => setExpanded(!expanded)}
          className="gap-1"
        >
          More Filters
          {expanded ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />}
        </Button>

        <FilterPresets />
      </div>

      {expanded && (
        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={state.model || "__all__"}
            onValueChange={(v) => handleSelectChange('model', v)}
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Model" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All Models</SelectItem>
              {modelOptions.map((m) => (
                <SelectItem key={m} value={m}>{m}</SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select
            value={state.status || "__all__"}
            onValueChange={(v) => handleSelectChange('status', v)}
          >
            <SelectTrigger className="w-[130px]">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All Status</SelectItem>
              {statusOptions.map((code) => (
                <SelectItem key={code} value={code}>{code}</SelectItem>
              ))}
            </SelectContent>
          </Select>

          {showScore && (
            <Select
              value={state.score || "__all__"}
              onValueChange={(v) => handleSelectChange('score', v)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue placeholder="Score" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All Scores</SelectItem>
                <SelectItem value="1">1</SelectItem>
                <SelectItem value="2">2</SelectItem>
                <SelectItem value="3">3</SelectItem>
                <SelectItem value="4">4</SelectItem>
                <SelectItem value="5">5</SelectItem>
                <SelectItem value="unscored">Unscored</SelectItem>
              </SelectContent>
            </Select>
          )}
        </div>
      )}

      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search..."
            value={keywordInput}
            onChange={(e) => setKeywordInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSearch();
            }}
            className="pl-8"
          />
          {keywordInput && (
            <button
              type="button"
              onClick={() => {
                setKeywordInput("");
                dispatch({ type: 'SET_KEYWORD', payload: "" });
                onFilterChange();
              }}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
        <Button size="sm" onClick={handleSearch}>
          Search
        </Button>
        {hasActiveFilters && (
          <Button
            variant="ghost"
            size="sm"
            className="gap-1 text-muted-foreground"
            onClick={handleClearFilters}
          >
            <X className="size-3.5" />
            Clear
          </Button>
        )}
      </div>

      <FilterTags onFilterChange={onFilterChange} />
    </div>
  );
}
```

- [ ] **Step 2: 提交代码**

```bash
git add web/src/components/filters/mobile-filter.tsx
git commit -m "feat: add mobile filter component"
```

### Task 6: 重构 Audit 页面

**Files:**
- Modify: `web/src/app/(dashboard)/audit/page.tsx`

- [ ] **Step 1: 重构 Audit 页面**

重构 `web/src/app/(dashboard)/audit/page.tsx`，使用新的筛选组件：

```typescript
"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import type { AuditLogItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ChevronLeft,
  ChevronRight,
  ScrollText,
  ListFilter,
  Check,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { FilterProvider, useFilter } from "@/lib/filter-context";
import { FilterToolbar } from "@/components/filters/filter-toolbar";
import { computeRange } from "@/lib/time-range";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)} / ${fmt(output)}`;
}

function formatCacheTokens(write: number, read: number): string | null {
  if (write === 0 && read === 0) return null;
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `c: ${fmt(read)} / ${fmt(write)}`;
}

function buildAuditFilter(user?: string, model?: string, status?: string): string | undefined {
  const parts: string[] = [];
  if (user) parts.push(`user:${user}`);
  if (model) parts.push(`model:${model}`);
  if (status) parts.push(`status:${status}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
}

function AuditPageContent() {
  const isMobile = useIsMobile();
  const { state } = useFilter();
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [pageInputValue, setPageInputValue] = useState("1");
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [userOptions, setUserOptions] = useState<string[]>([]);
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [statusOptions, setStatusOptions] = useState<string[]>([]);

  const fetchLogs = useCallback(
    async (page: number, pageSize: number) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(state.timeRange, state.customStart, state.customEnd);
        const filter = buildAuditFilter(state.user || undefined, state.model || undefined, state.status || undefined);
        const rsp = await api.listAuditLogs({
          page,
          pageSize,
          query: state.keyword || undefined,
          startTime,
          endTime,
          filter,
        });
        if (rsp.error) {
          toast.error(rsp.error.message ?? "Failed to load audit logs");
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPageInputValue(String(rsp.pageInfo.page));
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to load audit logs");
      } finally {
        setLoading(false);
      }
    },
    [state],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Initial data fetch on mount */
  useEffect(() => {
    fetchLogs(1, 20);
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect */

  useEffect(() => {
    api.listAuditOptions({ field: "user" }).then((rsp) => {
      if (!rsp.error && rsp.items) setUserOptions(rsp.items);
    }).catch((err) => {
      console.error("Failed to load user options:", err);
      toast.error("Failed to load user filter options");
    });
    api.listAuditOptions({ field: "model" }).then((rsp) => {
      if (!rsp.error && rsp.items) setModelOptions(rsp.items);
    }).catch((err) => {
      console.error("Failed to load model options:", err);
      toast.error("Failed to load model filter options");
    });
    api.listAuditOptions({ field: "status" }).then((rsp) => {
      if (!rsp.error && rsp.items) setStatusOptions(rsp.items);
    }).catch((err) => {
      console.error("Failed to load status options:", err);
      toast.error("Failed to load status filter options");
    });
  }, []);

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo],
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize);

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    navigator.clipboard.writeText(traceId).then(
      () => toast.success("TraceID copied"),
      () => toast.error("Copy failed"),
    );
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
          Audit
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          Inspect model call records, latency, errors, and trace IDs.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">Audit Logs</CardTitle>
        </CardHeader>
        <CardContent>
          <FilterToolbar
            userOptions={userOptions}
            modelOptions={modelOptions}
            statusOptions={statusOptions}
            onFilterChange={() => refresh(1)}
          />

          {/* 列表 */}
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">No audit logs in selected range</p>
            </div>
          ) : isMobile ? (
            <div className="space-y-3">
              {logs.map((log) => {
                const ok = log.upstreamStatusCode === 200;
                const hasError = !!log.errorMessage;
                const isExpanded = expandedId === log.id;
                const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);

                return (
                  <div
                    key={log.id}
                    className={`rounded-lg border border-border bg-card ${ok ? "" : "bg-destructive/5"}`}
                  >
                    <div
                      className="cursor-pointer p-4"
                      onClick={() => setExpandedId(isExpanded ? null : log.id)}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium">{log.model || "—"}</p>
                          <p className="mt-0.5 truncate text-xs text-muted-foreground">
                            {log.userName || "—"} · {log.apiKeyName || "—"}
                          </p>
                        </div>
                        <div className="shrink-0 text-right">
                          <Badge
                            variant={ok ? "secondary" : "destructive"}
                            className="text-xs"
                          >
                            {log.upstreamStatusCode}
                          </Badge>
                          {hasError && (
                            <p className="mt-1 max-w-[200px] truncate text-xs text-destructive">
                              {log.errorMessage}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                        <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                        <span>I: {log.firstTokenLatencyMs}ms</span>
                        {log.streamDurationMs > 0 && (
                          <span>O: {(log.streamDurationMs / 1000).toFixed(1)}s</span>
                        )}
                        {cacheInfo && <span>{cacheInfo}</span>}
                        <span
                          className="cursor-pointer font-mono underline-offset-2 hover:underline"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleCopyTrace(log.traceId);
                          }}
                          title="Click to copy full traceID"
                        >
                          {log.traceId.slice(-6) || "—"}
                        </span>
                      </div>
                      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground/70">
                        <span>{formatTime(log.createdAt)}</span>
                        <span
                          className="inline-block transition-transform duration-200 motion-reduce:transition-none"
                          style={{ transform: isExpanded ? "rotate(180deg)" : "rotate(0deg)" }}
                        >
                          ▾
                        </span>
                      </div>
                    </div>

                    <div
                      className="grid overflow-hidden transition-all duration-[250ms] ease-out motion-reduce:transition-none"
                      style={{ gridTemplateRows: isExpanded ? "1fr" : "0fr" }}
                    >
                      <div className="min-h-0">
                        <div className="border-t border-border px-4 pb-4 pt-3">
                          {hasError && (
                            <div className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-xs">
                              <span className="font-medium text-destructive">Error: </span>
                              <span className="text-destructive">{log.errorMessage}</span>
                            </div>
                          )}

                          <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                            <div>
                              <span className="text-muted-foreground">Input Tokens</span>
                              <p>{log.inputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Output Tokens</span>
                              <p>{log.outputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Cache Read</span>
                              <p>{log.cacheReadInputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Cache Creation</span>
                              <p>{log.cacheCreationInputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">I (First Token)</span>
                              <p>{log.firstTokenLatencyMs}ms</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">O (Stream Duration)</span>
                              <p>{log.streamDurationMs > 0 ? `${(log.streamDurationMs / 1000).toFixed(1)}s` : "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Upstream</span>
                              <p>{log.upstreamProtocol || "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Endpoint</span>
                              <p>{log.endpoint || "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">User</span>
                              <p>{log.userName || "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">API Protocol</span>
                              <p>{log.apiProtocol || "—"}</p>
                            </div>
                          </div>

                          <div className="mt-3 border-t border-border pt-2 text-xs">
                            <span className="text-muted-foreground">UA: </span>
                            <span className="break-all">{log.userAgent || "—"}</span>
                          </div>

                          <div className="mt-2 flex items-center justify-between border-t border-border pt-2 text-xs">
                            <span className="text-muted-foreground">
                              {formatTime(log.createdAt)}
                            </span>
                            <span
                              className="cursor-pointer font-mono text-muted-foreground underline-offset-2 hover:underline"
                              onClick={() => handleCopyTrace(log.traceId)}
                            >
                              Copy TraceID
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Endpoint</TableHead>
                  <TableHead>Protocol</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>UserAgent</TableHead>
                  <TableHead>TraceID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  const hasError = !!log.errorMessage;
                  const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);
                  const uaShort = log.userAgent ? log.userAgent.slice(0, 30) + (log.userAgent.length > 30 ? "…" : "") : "—";
                  return (
                    <TableRow
                      key={log.id}
                      className={ok ? "" : "bg-destructive/5"}
                    >
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatTime(log.createdAt)}
                      </TableCell>
                      <TableCell className="max-w-[180px] truncate">{log.model || "—"}</TableCell>
                      <TableCell className="max-w-[140px] truncate text-muted-foreground">{log.endpoint || "—"}</TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div className="text-xs">{log.apiProtocol || "—"}</div>
                        <div className="text-xs text-muted-foreground/70">{log.upstreamProtocol || "—"}</div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">{log.userName || "—"}</div>
                        <div className="text-xs text-muted-foreground">
                          {log.apiKeyName || "—"}
                        </div>
                      </TableCell>
                      <TableCell>
                        {!ok && hasError ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button">
                                    <Badge variant="destructive" className="text-xs">
                                      {log.upstreamStatusCode}
                                    </Badge>
                                  </button>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs">
                                <span>{log.errorMessage}</span>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <Badge
                            variant={ok ? "secondary" : "destructive"}
                            className="text-xs"
                          >
                            {log.upstreamStatusCode}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div>{formatTokens(log.inputTokens, log.outputTokens)}</div>
                        {cacheInfo && (
                          <div className="text-xs text-muted-foreground">{cacheInfo}</div>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div className="text-xs">I: {log.firstTokenLatencyMs}ms</div>
                        {log.streamDurationMs > 0 && (
                          <div className="text-xs">O: {(log.streamDurationMs / 1000).toFixed(1)}s</div>
                        )}
                      </TableCell>
                      <TableCell>
                        {log.userAgent ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button" className="max-w-[160px] cursor-default truncate text-xs text-muted-foreground">
                                    {uaShort}
                                  </button>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs">
                                <span className="break-all">{log.userAgent}</span>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell
                        className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}

          {/* 分页 */}
          {pageInfo.total > 0 && (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
              <div className="hidden items-center gap-3 md:flex">
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={<Button variant="outline" size="sm" className="gap-1.5" />}
                  >
                    <ListFilter className="size-3.5" />
                    {pageInfo.pageSize} / page
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {[20, 50, 100].map((size) => (
                      <DropdownMenuItem key={size} onClick={() => refresh(1, size)}>
                        {size === pageInfo.pageSize && <Check className="size-4" />}
                        <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>
                          {size} per page
                        </span>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <p className="hidden text-sm text-muted-foreground md:block">
                  {pageInfo.total} log{pageInfo.total !== 1 ? "s" : ""} total
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={pageInfo.page <= 1}
                  onClick={() => refresh(pageInfo.page - 1)}
                >
                  <ChevronLeft className="size-4" />
                </Button>
                <div className="flex items-center gap-1.5 text-sm">
                  <span className="text-muted-foreground">Page</span>
                  <input
                    type="number"
                    min={1}
                    max={totalPages}
                    value={pageInputValue}
                    onChange={(e) => setPageInputValue(e.target.value)}
                    className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        let page = parseInt(pageInputValue, 10);
                        if (Number.isNaN(page)) page = 1;
                        page = Math.max(1, Math.min(page, totalPages));
                        refresh(page);
                      }
                    }}
                    onBlur={() => {
                      let page = parseInt(pageInputValue, 10);
                      if (Number.isNaN(page)) page = 1;
                      page = Math.max(1, Math.min(page, totalPages));
                      refresh(page);
                    }}
                  />
                  <span className="text-muted-foreground">/ {totalPages}</span>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={pageInfo.page >= totalPages}
                  onClick={() => refresh(pageInfo.page + 1)}
                >
                  <ChevronRight className="size-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default function AuditPage() {
  return (
    <FilterProvider>
      <AuditPageContent />
    </FilterProvider>
  );
}
```

- [ ] **Step 2: 提交代码**

```bash
git add web/src/app/(dashboard)/audit/page.tsx
git commit -m "refactor: use new filter components in audit page"
```

### Task 7: 重构 Sessions 页面

**Files:**
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 重构 Sessions 页面**

重构 `web/src/app/(dashboard)/sessions/page.tsx`，使用新的筛选组件：

由于 Sessions 页面的重构与 Audit 页面类似，但需要添加评分筛选功能，这里提供关键的修改点：

1. 导入新的筛选组件
2. 使用 FilterProvider 包裹页面内容
3. 使用 FilterToolbar 替换原有的筛选逻辑
4. 添加 showScore={true} 属性
5. 移除原有的筛选状态管理代码

- [ ] **Step 2: 提交代码**

```bash
git add web/src/app/(dashboard)/sessions/page.tsx
git commit -m "refactor: use new filter components in sessions page"
```

### Task 8: 测试和验证

**Files:**
- Test: `web/src/components/filters/__tests__/filter-toolbar.test.tsx`

- [ ] **Step 1: 运行前端 lint 检查**

```bash
cd web && npm run lint
```

Expected: No errors

- [ ] **Step 2: 运行前端构建**

```bash
cd web && npm run build
```

Expected: Build successful

- [ ] **Step 3: 运行后端测试**

```bash
make test
```

Expected: All tests pass

- [ ] **Step 4: 运行后端 lint 检查**

```bash
make lint
```

Expected: All checks passed

- [ ] **Step 5: 提交最终代码**

```bash
git add .
git commit -m "feat: complete audit and session filter redesign"
```

## 验收标准

1. **视觉体验**：符合 claude.ai 的现代设计风格
2. **筛选效率**：支持筛选预设和实时筛选
3. **移动端体验**：响应式设计，折叠式布局
4. **性能**：筛选响应时间 < 500ms
5. **可访问性**：符合 WCAG 2.1 AA 标准
6. **兼容性**：支持主流浏览器（Chrome, Firefox, Safari, Edge）

## 注意事项

1. 保持现有的 API 接口不变
2. 保持现有的功能完整性
3. 遵循项目的代码规范
4. 确保移动端和桌面端的兼容性
5. 注意性能优化，避免不必要的重新渲染