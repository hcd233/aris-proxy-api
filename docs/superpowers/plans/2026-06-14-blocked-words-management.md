# Blocked Words Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add sensitive word blacklist (blocked words) management API + content filtering in LLM proxy + hit count tracking + web UI tab

**Architecture:** DDD-lite following existing endpoint CRUD pattern. Aho-Corasick automaton for O(n) substring matching in memory. Redis atomic counters + cron batch-sync for hit count persistence.

**Tech Stack:** Go 1.25, GORM, Aho-Corasick (in-house), Redis, Fiber, Huma, Next.js 16, shadcn/ui

---

### Task 1: DB Model + DAO + Constants

**Files:**
- Create: `internal/infrastructure/database/model/blocked.go`
- Create: `internal/infrastructure/database/dao/blocked.go`
- Modify: `internal/infrastructure/database/model/base.go` (register model)
- Modify: `internal/infrastructure/database/dao/singleton.go` (register DAO)
- Modify: `internal/common/constant/sql.go` (field names)
- Modify: `internal/common/constant/string.go` (tag name)
- Modify: `internal/common/constant/cron.go` (cron module name + spec)

- [ ] **Step 1: Create DB model** `internal/infrastructure/database/model/blocked.go`

```go
package model

type Blocked struct {
	BaseModel
	Word     string `json:"word" gorm:"column:word;type:varchar(512);not null;uniqueIndex:idx_word_deleted_at,priority:1;comment:敏感词"`
	HitCount uint   `json:"hit_count" gorm:"column:hit_count;not null;default:0;comment:命中次数"`
}

func (Blocked) TableName() string {
	return "blocked_words"
}
```

- [ ] **Step 2: Create DAO** `internal/infrastructure/database/dao/blocked.go`

```go
package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type BlockedDAO struct {
	baseDAO[dbmodel.Blocked]
}
```

- [ ] **Step 3: Register in base.go** — add `&Blocked{}` to the `Models` slice

```go
var Models = []any{
	&User{},
	&Message{},
	&Session{},
	&Tool{},
	&Endpoint{},
	&Model{},
	&ProxyAPIKey{},
	&ModelCallAudit{},
	&Blocked{},
}
```

- [ ] **Step 4: Register DAO singleton** — add to `internal/infrastructure/database/dao/singleton.go`

Add to the var block:
```go
blockedDAOSingleton *BlockedDAO
```

Add to init():
```go
blockedDAOSingleton = &BlockedDAO{}
```

Add getter:
```go
func GetBlockedDAO() *BlockedDAO {
	return blockedDAOSingleton
}
```

- [ ] **Step 5: Add SQL field constants** to `internal/common/constant/sql.go`

```go
FieldWord     = "word"
FieldHitCount = "hit_count"
```

And field slices:
```go
BlockedRepoFieldsFull = []string{FieldID, FieldWord, FieldHitCount, FieldCreatedAt, FieldUpdatedAt}
```

- [ ] **Step 6: Add tag constant** to `internal/common/constant/string.go`

```go
TagBlocked = "Blocked"
```

- [ ] **Step 7: Add cron constants** to `internal/common/constant/cron.go`

```go
CronModuleBlockedHitSync = "BlockedHitSyncCron"
CronSpecBlockedHitSync   = "*/5 * * * *"
```

---

### Task 2: Domain Aggregate + Repository Interface

**Files:**
- Create: `internal/domain/blocked/aggregate/blocked.go`
- Create: `internal/domain/blocked/repository.go`

- [ ] **Step 1: Create domain aggregate** `internal/domain/blocked/aggregate/blocked.go`

```go
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
)

type Blocked struct {
	commonagg.Base
	word      string
	hitCount  uint
	createdAt time.Time
	updatedAt time.Time
}

func CreateBlocked(id uint, word string) (*Blocked, error) {
	if word == "" {
		return nil, ierr.New(ierr.ErrValidation, "blocked word cannot be empty")
	}
	b := &Blocked{word: word}
	b.SetID(id)
	return b, nil
}

func (*Blocked) AggregateType() string { return enum.AggregateTypeBlocked }

func (b *Blocked) Word() string                { return b.word }
func (b *Blocked) HitCount() uint              { return b.hitCount }
func (b *Blocked) CreatedAt() time.Time         { return b.createdAt }
func (b *Blocked) UpdatedAt() time.Time         { return b.updatedAt }
func (b *Blocked) SetTimestamps(createdAt, updatedAt time.Time) {
	b.createdAt = createdAt
	b.updatedAt = updatedAt
}
```

- [ ] **Step 2: Add enum** `AggregateTypeBlocked` to `internal/common/enum`

Find `internal/common/enum/` directory, read files, add in the appropriate enum file:

```go
AggregateTypeBlocked AggregateType = "blocked"
```

- [ ] **Step 3: Create repository interface** `internal/domain/blocked/repository.go`

```go
package blocked

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type BlockedRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error)
	Create(ctx context.Context, word *aggregate.Blocked) (uint, error)
	Delete(ctx context.Context, id uint) error
	Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error)
	ListAll(ctx context.Context) ([]*aggregate.Blocked, error)
	BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error
}
```

---

### Task 3: Error Sentinel

**Files:**
- Modify: `internal/common/ierr/sentinels.go`

- [ ] **Step 1: Add `ErrContentBlocked` sentinel**

```go
// ErrContentBlocked 内容违反策略
ErrContentBlocked = newFromSentinel(newSentinel("content_blocked", model.NewError(10010, "ContentBlocked")))
```

Place it after `ErrResourceLocked` in the "通用错误" section. HTTP status code mapping: in `api/util/http.go`, check how errors map to HTTP status. The sentinel's biz error code 10010 should map to 403. Look at `ierr.ToBizError` to verify the default mapping, or add an HTTP status override.

---

### Task 4: DTOs

**Files:**
- Create: `internal/dto/blocked.go`

- [ ] **Step 1: Create DTO file** `internal/dto/blocked.go`

```go
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateBlockedReq struct {
	Body *CreateBlockedReqBody `json:"body" doc:"Request body"`
}

type CreateBlockedReqBody struct {
	Word string `json:"word" required:"true" minLength:"1" maxLength:"512" doc:"敏感词"`
}

type DeleteBlockedReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Blocked ID"`
}

type ListBlockedReq struct {
	model.CommonParam
}

type ListBlockedRsp struct {
	CommonRsp
	Blocked  []*BlockedItem  `json:"blocked,omitempty" doc:"Blocked 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

type BlockedItem struct {
	ID        uint      `json:"id" doc:"Blocked ID"`
	Word      string    `json:"word" doc:"敏感词"`
	HitCount  uint      `json:"hitCount" doc:"命中次数"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
}
```

---

### Task 5: Aho-Corasick Matcher

**Files:**
- Create: `internal/application/blocked/matcher.go`

- [ ] **Step 1: Implement AC automaton**

```go
package blocked

type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	output   []uint // word IDs that end at this node
}

// ACmatcher Aho-Corasick 多模式匹配器
type ACmatcher struct {
	root *acNode
}

// NewACmatcher builds an AC automaton from word→id mappings.
func NewACmatcher(words map[uint]string) *ACmatcher {
	m := &ACmatcher{root: &acNode{children: make(map[rune]*acNode)}}
	// Build trie
	for id, word := range words {
		node := m.root
		for _, r := range word {
			child, ok := node.children[r]
			if !ok {
				child = &acNode{children: make(map[rune]*acNode)}
				node.children[r] = child
			}
			node = child
		}
		node.output = append(node.output, id)
	}
	// Build fail pointers (BFS)
	queue := make([]*acNode, 0)
	for _, child := range m.root.children {
		child.fail = m.root
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for r, child := range parent.children {
			fail := parent.fail
			for fail != nil {
				if next, ok := fail.children[r]; ok {
					child.fail = next
					break
				}
				fail = fail.fail
			}
			if child.fail == nil {
				child.fail = m.root
			}
			child.output = append(child.output, child.fail.output...)
			queue = append(queue, child)
		}
	}
	return m
}

// Match returns all matched word IDs found in text.
func (m *ACmatcher) Match(text string) []uint {
	matched := make(map[uint]struct{})
	node := m.root
	for _, r := range text {
		for node != m.root && node.children[r] == nil {
			node = node.fail
		}
		if next, ok := node.children[r]; ok {
			node = next
		}
		for _, id := range node.output {
			matched[id] = struct{}{}
		}
	}
	result := make([]uint, 0, len(matched))
	for id := range matched {
		result = append(result, id)
	}
	return result
}
```

---

### Task 6: BlockedService

**Files:**
- Create: `internal/application/blocked/service.go`

- [ ] **Step 1: Create BlockedService**

```go
package blocked

import (
	"context"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type BlockedService struct {
	mu       sync.RWMutex
	matcher  *ACmatcher
	wordIDs  map[string]uint // word → id for hit counting
	repo     blocked.BlockedRepository
}

func NewBlockedService(repo blocked.BlockedRepository) *BlockedService {
	s := &BlockedService{repo: repo}
	s.rebuild()
	return s
}

func (s *BlockedService) rebuild() {
	words := make(map[uint]string)
	ids := make(map[string]uint)
	// This is called during init and on CRUD changes.
	// For production, use real context; for init use context.Background()
	ctx := context.Background()
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		// Fallback to empty matcher on error
		s.matcher = NewACmatcher(words)
		return
	}
	for _, b := range all {
		words[b.AggregateID()] = b.Word()
		ids[b.Word()] = b.AggregateID()
	}
	s.matcher = NewACmatcher(words)
	s.wordIDs = ids
}

// Rebuild reloads the automaton from DB. Call after Create/Delete.
func (s *BlockedService) Rebuild() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuild()
}

// Check returns matched word IDs found in text.
func (s *BlockedService) Check(text string) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matcher.Match(text)
}
```

---

### Task 7: Redis Hit Counter

**Files:**
- Create: `internal/infrastructure/cache/blocked.go`

- [ ] **Step 1: Create Redis hit counter**

```go
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const blockedHitKeyPrefix = "blocked:hit:"

// BlockedHitCache manages hit count increments in Redis.
type BlockedHitCache struct {
	client *redis.Client
}

func NewBlockedHitCache(client *redis.Client) *BlockedHitCache {
	return &BlockedHitCache{client: client}
}

func blockedHitKey(id uint) string {
	return fmt.Sprintf("%s%d", blockedHitKeyPrefix, id)
}

// IncrementHits atomically increments hit counts for given word IDs.
func (c *BlockedHitCache) IncrementHits(ctx context.Context, ids []uint) error {
	pipe := c.client.Pipeline()
	for _, id := range ids {
		pipe.IncrBy(ctx, blockedHitKey(id), 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// PopAll reads all hit counters and clears them atomically.
// Returns map[wordID]count.
func (c *BlockedHitCache) PopAll(ctx context.Context) (map[uint]uint, error) {
	iter := c.client.Scan(ctx, 0, blockedHitKeyPrefix+"*", 0).Iterator()
	result := make(map[uint]uint)
	pipe := c.client.Pipeline()
	var keys []string

	for iter.Next(ctx) {
		key := iter.Val()
		keys = append(keys, key)
		pipe.Get(ctx, key)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return result, nil
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for i, key := range keys {
		if cmds[i].Err() != nil {
			continue
		}
		val, _ := cmds[i].(*redis.StringCmd).Uint64()
		if val > 0 {
			var id uint
			fmt.Sscanf(key, blockedHitKeyPrefix+"%d", &id)
			result[id] = uint(val)
		}
	}

	// Clear the keys
	pipe = c.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return result, err
	}

	return result, nil
}
```

---

### Task 8: Repository Implementation

**Files:**
- Create: `internal/infrastructure/repository/blocked_repository.go`
- Modify: `internal/common/constant/sql.go` (add field slice)

- [ ] **Step 1: Create repository** `internal/infrastructure/repository/blocked_repository.go`

```go
package repository

import (
	"context"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type blockedRepository struct {
	dao *dao.BlockedDAO
	db  *gorm.DB
}

func NewBlockedRepository(db *gorm.DB) blocked.BlockedRepository {
	return &blockedRepository{dao: dao.GetBlockedDAO(), db: db}
}

func (r *blockedRepository) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) {
	m := &dbmodel.Blocked{ID: id}
	err := r.dao.Get(r.db, m, constant.BlockedRepoFieldsFull)
	if err != nil {
		return nil, err
	}
	return toBlockedAggregate(m)
}

func (r *blockedRepository) Create(ctx context.Context, word *aggregate.Blocked) (uint, error) {
	m := toBlockedDBModel(word)
	err := r.dao.Create(r.db, m)
	return m.ID, err
}

func (r *blockedRepository) Delete(ctx context.Context, id uint) error {
	return r.dao.Delete(r.db, &dbmodel.Blocked{ID: id})
}

func (r *blockedRepository) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	fields := constant.BlockedRepoFieldsFull
	var dbModels []*dbmodel.Blocked
	pageInfo, err := r.dao.Paginate(r.db, &dbModels, fields, &dao.CommonParam{
		Page:     param.Page,
		PageSize: param.PageSize,
		Query:    param.Query,
		QueryFields: []string{constant.FieldWord},
		Order:    constant.FieldID + " DESC",
	})
	if err != nil {
		return nil, nil, err
	}
	items := lo.Map(dbModels, func(m *dbmodel.Blocked, _ int) *aggregate.Blocked {
		item, _ := toBlockedAggregate(m)
		return item
	})
	return items, pageInfo, nil
}

func (r *blockedRepository) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	var dbModels []*dbmodel.Blocked
	err := r.dao.FindAll(r.db, &dbModels, constant.BlockedRepoFieldsFull)
	if err != nil {
		return nil, err
	}
	return lo.Map(dbModels, func(m *dbmodel.Blocked, _ int) *aggregate.Blocked {
		item, _ := toBlockedAggregate(m)
		return item
	}), nil
}

func (r *blockedRepository) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	for id, count := range idHits {
		err := r.db.Model(&dbmodel.Blocked{}).
			Where(constant.WhereIDEquals, id).
			UpdateColumn(constant.FieldHitCount, gorm.Expr(constant.FieldHitCount+" + ?", count)).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func toBlockedAggregate(m *dbmodel.Blocked) (*aggregate.Blocked, error) {
	b, err := aggregate.CreateBlocked(m.ID, m.Word)
	if err != nil {
		return nil, err
	}
	b.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return b, nil
}

func toBlockedDBModel(b *aggregate.Blocked) *dbmodel.Blocked {
	return &dbmodel.Blocked{
		Word: b.Word(),
	}
}
```

---

### Task 9: Application Port + Commands + Queries

**Files:**
- Create: `internal/application/blocked/port/handler.go`
- Create: `internal/application/blocked/command/create_blocked.go`
- Create: `internal/application/blocked/command/delete_blocked.go`
- Create: `internal/application/blocked/query/list_blocked.go`

- [ ] **Step 1: Create port** `internal/application/blocked/port/handler.go`

```go
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateBlockedCommand struct {
	Word string
}

type CreateBlockedResult struct {
	BlockedID uint
}

type CreateBlockedHandler interface {
	Handle(ctx context.Context, cmd CreateBlockedCommand) (*CreateBlockedResult, error)
}

type DeleteBlockedCommand struct {
	BlockedID uint
}

type DeleteBlockedHandler interface {
	Handle(ctx context.Context, cmd DeleteBlockedCommand) error
}

type BlockedView struct {
	ID        uint
	Word      string
	HitCount  uint
	CreatedAt time.Time
}

type ListBlockedQuery struct {
	model.CommonParam
}

type ListBlockedHandler interface {
	Handle(ctx context.Context, q ListBlockedQuery) ([]*BlockedView, *model.PageInfo, error)
}
```

- [ ] **Step 2: Create command** `internal/application/blocked/command/create_blocked.go`

```go
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
)

type createBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func()
}

func NewCreateBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func()) port.CreateBlockedHandler {
	return &createBlockedHandler{repo: repo, rebuildNotify: rebuildNotify}
}

func (h *createBlockedHandler) Handle(ctx context.Context, cmd port.CreateBlockedCommand) (*port.CreateBlockedResult, error) {
	b, err := aggregate.CreateBlocked(0, cmd.Word)
	if err != nil {
		return nil, err
	}
	id, err := h.repo.Create(ctx, b)
	if err != nil {
		return nil, err
	}
	h.rebuildNotify()
	return &port.CreateBlockedResult{BlockedID: id}, nil
}
```

- [ ] **Step 3: Create delete command** `internal/application/blocked/command/delete_blocked.go`

```go
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
)

type deleteBlockedHandler struct {
	repo          blocked.BlockedRepository
	rebuildNotify func()
}

func NewDeleteBlockedHandler(repo blocked.BlockedRepository, rebuildNotify func()) port.DeleteBlockedHandler {
	return &deleteBlockedHandler{repo: repo, rebuildNotify: rebuildNotify}
}

func (h *deleteBlockedHandler) Handle(ctx context.Context, cmd port.DeleteBlockedCommand) error {
	err := h.repo.Delete(ctx, cmd.BlockedID)
	if err != nil {
		return err
	}
	h.rebuildNotify()
	return nil
}
```

- [ ] **Step 4: Create list query** `internal/application/blocked/query/list_blocked.go`

```go
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type listBlockedHandler struct {
	repo blocked.BlockedRepository
}

func NewListBlockedHandler(repo blocked.BlockedRepository) port.ListBlockedHandler {
	return &listBlockedHandler{repo: repo}
}

func (h *listBlockedHandler) Handle(ctx context.Context, q port.ListBlockedQuery) ([]*port.BlockedView, *model.PageInfo, error) {
	items, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		return nil, nil, err
	}
	views := lo.Map(items, func(b *aggregate.Blocked, _ int) *port.BlockedView {
		return &port.BlockedView{
			ID:        b.AggregateID(),
			Word:      b.Word(),
			HitCount:  b.HitCount(),
			CreatedAt: b.CreatedAt(),
		}
	})
	return views, pageInfo, nil
}
```

Wait — the query imports aggregate. Let me fix:

```go
import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)
```

---

### Task 10: Handler

**Files:**
- Create: `internal/handler/blocked.go`

- [ ] **Step 1: Create handler** `internal/handler/blocked.go`

```go
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type BlockedHandler interface {
	HandleCreateBlocked(ctx context.Context, req *dto.CreateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListBlocked(ctx context.Context, req *dto.ListBlockedReq) (*dto.HTTPResponse[*dto.ListBlockedRsp], error)
	HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type BlockedDependencies struct {
	Create port.CreateBlockedHandler
	Delete port.DeleteBlockedHandler
	List   port.ListBlockedHandler
}

type blockedHandler struct {
	create port.CreateBlockedHandler
	delete port.DeleteBlockedHandler
	list   port.ListBlockedHandler
}

func NewBlockedHandler(deps BlockedDependencies) BlockedHandler {
	return &blockedHandler{
		create: deps.Create,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *blockedHandler) HandleCreateBlocked(ctx context.Context, req *dto.CreateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, port.CreateBlockedCommand{
		Word: req.Body.Word,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Create blocked word failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	_ = result.BlockedID
	logger.WithCtx(ctx).Info("[BlockedHandler] Create blocked word success",
		zap.Uint("userID", userID), zap.String("word", req.Body.Word))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *blockedHandler) HandleListBlocked(ctx context.Context, req *dto.ListBlockedReq) (*dto.HTTPResponse[*dto.ListBlockedRsp], error) {
	rsp := &dto.ListBlockedRsp{}

	views, pageInfo, err := h.list.Handle(ctx, port.ListBlockedQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] List blocked words failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Blocked = lo.Map(views, func(v *port.BlockedView, _ int) *dto.BlockedItem {
		return &dto.BlockedItem{
			ID:        v.ID,
			Word:      v.Word,
			HitCount:  v.HitCount,
			CreatedAt: v.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *blockedHandler) HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, port.DeleteBlockedCommand{BlockedID: req.ID})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Delete blocked word failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

---

### Task 11: Router

**Files:**
- Create: `internal/router/blocked.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Create router** `internal/router/blocked.go`

```go
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initBlockedRouter(group huma.API, handler handler.BlockedHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	group.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(group, huma.Operation{
		OperationID: "createBlocked",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "CreateBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleCreateBlocked)

	huma.Register(group, huma.Operation{
		OperationID: "listBlocked",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleListBlocked)

	huma.Register(group, huma.Operation{
		OperationID: "deleteBlocked",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteBlocked",
		Tags:        []string{constant.TagBlocked},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteBlocked", enum.PermissionAdmin),
		},
	}, handler.HandleDeleteBlocked)
}
```

Wait — the delete path uses `{id}` but the DTO uses `query:"id"`. Let me check how delete endpoint does it. Looking at `DeleteEndpointReq`:

```go
type DeleteEndpointReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
}
```

And in the router for endpoint:
```go
huma.Register(group, huma.Operation{
	OperationID: "deleteEndpoint",
	Method:      http.MethodDelete,
	Path:        "/{id}",
	...
}, handler.HandleDeleteEndpoint)
```

So the path uses `{id}` which Huma matches to the `query:"id"` tag. That's fine.

Actually wait, there's a subtlety - `{id}` in path. But Huma also has `query:"id"`. Let me look at how huma handles this. Actually in Huma v2, the path parameters are defined in the Path field with `{param}` syntax, and the struct fields are matched by name with the `path:"paramName"` tag. But the existing code uses `query:"id"` for delete operations. So Huma must be able to extract the `id` from either query or path.

Let me just match the existing pattern exactly. Looking at the endpoint router: `Path: "/{id}"` and the DeleteEndpointReq has `query:"id"`. This works in Huma because Huma will look for `id` in path params, query params, etc. based on the struct tags. The `query` tag is the fallback.

Actually, for path parameters, Huma uses `param:"id"` or `path:"id"` tag. But the existing code uses `query:"id"` with a `{id}` path segment. Let me check if this works... In Huma v2, parameter binding uses the `path` tag for path params and `query` for query params. If the param is defined with query but the actual value comes from the path, it might work due to some fallback behavior. But I'm not 100% sure.

Let me just follow the exact convention. The endpoint delete works, so I'll use the same pattern.

- [ ] **Step 2: Modify `internal/router/router.go`**

Add `BlockedHandler` to `APIRouterDependencies`:
```go
type APIRouterDependencies struct {
	...
	BlockedHandler   handler.BlockedHandler
}
```

Add import for `jwt` package (or use existing import). Actually, `jwt` is already imported as `github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt`.

Register the group in `RegisterAPIRouter`:
```go
blockedGroup := huma.NewGroup(v1Group, "/block")
initBlockedRouter(blockedGroup, deps.BlockedHandler, deps.DB, deps.Cache, deps.AccessSigner)
```

---

### Task 12: Bootstrap Wiring

**Files:**
- Modify: `internal/bootstrap/modules/repository.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`
- Modify: `internal/bootstrap/router.go`
- Modify: `internal/bootstrap/modules/cron.go`
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: Repository module** — add `NewBlockedRepository`

In `internal/bootstrap/modules/repository.go`:
```go
import blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
```

Add to `RepositoryModule` `fx.Provide`:
```go
NewBlockedRepository,
```

Add function:
```go
func NewBlockedRepository(db *gorm.DB) blockeddomain.BlockedRepository {
	return repository.NewBlockedRepository(db)
}
```

- [ ] **Step 2: Application module** — add blocked handlers + service

In `internal/bootstrap/modules/application.go`:

Imports:
```go
blockedcommand "github.com/hcd233/aris-proxy-api/internal/application/blocked/command"
blockedport "github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
blockedquery "github.com/hcd233/aris-proxy-api/internal/application/blocked/query"
blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"
blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
```

Add to `ApplicationModule` `fx.Provide`:
```go
NewBlockedService,
NewCreateBlockedHandler,
NewDeleteBlockedHandler,
NewListBlockedHandler,
```

Add functions:
```go
func NewBlockedService(repo blockeddomain.BlockedRepository) *blockedapp.BlockedService {
	return blockedapp.NewBlockedService(repo)
}

func NewCreateBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.CreateBlockedHandler {
	return blockedcommand.NewCreateBlockedHandler(repo, svc.Rebuild)
}

func NewDeleteBlockedHandler(repo blockeddomain.BlockedRepository, svc *blockedapp.BlockedService) blockedport.DeleteBlockedHandler {
	return blockedcommand.NewDeleteBlockedHandler(repo, svc.Rebuild)
}

func NewListBlockedHandler(repo blockeddomain.BlockedRepository) blockedport.ListBlockedHandler {
	return blockedquery.NewListBlockedHandler(repo)
}
```

- [ ] **Step 3: Handler module** — add blocked handler

In `internal/bootstrap/modules/handler.go`:

Imports:
```go
blockedport "github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
```

Add to `HandlerModule` `fx.Provide`:
```go
NewBlockedDependencies,
handler.NewBlockedHandler,
```

Add function:
```go
func NewBlockedDependencies(create blockedport.CreateBlockedHandler, delete blockedport.DeleteBlockedHandler, list blockedport.ListBlockedHandler) handler.BlockedDependencies {
	return handler.BlockedDependencies{
		Create: create,
		Delete: delete,
		List:   list,
	}
}
```

- [ ] **Step 4: Router bootstrap** — add blocked handler to route params

In `internal/bootstrap/router.go`:

Add to `routeParams`:
```go
BlockedHandler   handler.BlockedHandler
```

Add to `RegisterAPIRouter` call:
```go
BlockedHandler:    params.BlockedHandler,
```

- [ ] **Step 5: Cron module** — add blocked hit sync cron

In `internal/bootstrap/modules/cron.go`:

Imports:
```go
blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"
blockedcache "github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
blockeddomain "github.com/hcd233/aris-proxy-api/internal/domain/blocked"
appcron "github.com/hcd233/aris-proxy-api/internal/cron"
```

Add to `CronModule` `fx.Provide`:
```go
NewBlockedHitSyncCron,
```

Add function:
```go
func NewBlockedHitSyncCron(db *gorm.DB, cache *redis.Client, blockedRepo blockeddomain.BlockedRepository, blockedHitCache *blockedcache.BlockedHitCache) appcron.Cron {
	return cron.NewBlockedHitSyncCron(db, blockedRepo, blockedHitCache)
}
```

Wait — the existing cron module doesn't have an explicit entry for each cron. Actually looking at `cron.go`, the cron entries are built in `buildRegistryEntries()` inside the cron package. The module just provides `NewCronEntries` which calls `InitCronJobs`.

So I need to modify `internal/cron/cron.go` to add the blocked hit sync entry to `buildRegistryEntries()`. Let me adjust:

In `internal/cron/cron.go`, add to `buildRegistryEntries()`:
```go
{
	Name:    constant.CronModuleBlockedHitSync,
	Enabled: func() bool { return config.CronBlockedHitSyncEnabled },
	Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
		blockedRepo := repository.NewBlockedRepository(db)
		blockedHitCache := cachepkg.NewBlockedHitCache(cache)
		return NewBlockedHitSyncCron(db, blockedRepo, blockedHitCache)
	},
},
```

And in `internal/config/config.go`, add:
```go
// CronBlockedHitSyncEnabled bool 是否启用敏感词命中计数同步定时任务
CronBlockedHitSyncEnabled bool
```

And in the init block:
```go
CronBlockedHitSyncEnabled = config.GetBool("cron.blocked.hit.sync.enabled")
```

And in `env/api.env.template`:
```
CRON_BLOCKED_HIT_SYNC_ENABLED=true
```

- [ ] **Step 6: Create Cron instance** `internal/cron/blocked_hit_sync.go`

```go
package cron

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"gorm.io/gorm"
)

type blockedHitSyncCron struct {
	cron        *cron.Cron
	db          *gorm.DB
	blockedRepo blocked.BlockedRepository
	hitCache    *cache.BlockedHitCache
}

func NewBlockedHitSyncCron(db *gorm.DB, blockedRepo blocked.BlockedRepository, hitCache *cache.BlockedHitCache) Cron {
	return &blockedHitSyncCron{
		cron:        cron.New(cron.WithSeconds(), cron.WithLogger(newCronLoggerAdapter(constant.CronModuleBlockedHitSync))),
		db:          db,
		blockedRepo: blockedRepo,
		hitCache:    hitCache,
	}
}

func (c *blockedHitSyncCron) Start() error {
	_, err := c.cron.AddFunc(constant.CronSpecBlockedHitSync, wrapCronFunc("BlockedHitSync", func(ctx context.Context) {
		c.sync(ctx)
	}))
	if err != nil {
		return err
	}
	c.cron.Start()
	return nil
}

func (c *blockedHitSyncCron) Stop() {
	<-c.cron.Stop().Done()
}

func (c *blockedHitSyncCron) sync(ctx context.Context) {
	hits, err := c.hitCache.PopAll(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to pop hit counts", zap.Error(err))
		return
	}
	if len(hits) == 0 {
		return
	}
	err = c.blockedRepo.BatchIncrementHitCount(ctx, hits)
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHitSync] Failed to batch increment hit counts", zap.Error(err))
		return
	}
	logger.WithCtx(ctx).Info("[BlockedHitSync] Synced hit counts",
		zap.Int("count", len(hits)))
}
```

- [ ] **Step 7: Add `CronBlockedHitSyncEnabled` config**

In `internal/config/config.go`:
```go
// CronBlockedHitSyncEnabled bool 是否启用敏感词命中计数同步定时任务
CronBlockedHitSyncEnabled bool
```
Add to init:
```go
CronBlockedHitSyncEnabled = config.GetBool("cron.blocked.hit.sync.enabled")
```

In `env/api.env.template`:
```
CRON_BLOCKED_HIT_SYNC_ENABLED=true
```

---

### Task 13: LLM Proxy Integration — Content Check

**Files:**
- Modify: `internal/application/llmproxy/usecase/port.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`

- [ ] **Step 1: Add BlockedService port interface**

In `internal/application/llmproxy/usecase/port.go`, add:
```go
// BlockedChecker 敏感词检查端口
type BlockedChecker interface {
	Check(text string) []uint
}
```

- [ ] **Step 2: Add helper function for extracting text content from messages**

Create `internal/application/llmproxy/usecase/blocked_check.go`:

```go
package usecase

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func extractOpenAIChatText(req *dto.OpenAIChatCompletionRequest) string {
	var buf strings.Builder
	for _, msg := range req.Body.Messages {
		if msg.Content != nil {
			if msg.Content.Text != "" {
				buf.WriteString(msg.Content.Text)
			}
			for _, part := range msg.Content.Parts {
				if part.Text != nil {
					buf.WriteString(*part.Text)
				}
			}
		}
		if msg.ReasoningContent != nil {
			buf.WriteString(*msg.ReasoningContent)
		}
	}
	return buf.String()
}

func extractAnthropicMessageText(req *dto.AnthropicCreateMessageRequest) string {
	var buf strings.Builder
	for _, msg := range req.Body.Messages {
		if msg.Content != nil {
			if msg.Content.Text != "" {
				buf.WriteString(msg.Content.Text)
			}
			for _, block := range msg.Content.Blocks {
				if block.Text != nil {
					buf.WriteString(*block.Text)
				}
				if block.Thinking != nil {
					buf.WriteString(*block.Thinking)
				}
			}
		}
	}
	return buf.String()
}
```

- [ ] **Step 3: Add checkContent method to usecase**

In `internal/application/llmproxy/usecase/blocked_check.go`:

```go
func (u *openAIUseCase) checkContent(ctx context.Context, req *dto.OpenAIChatCompletionRequest) error {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractOpenAIChatText(req)
	matched := u.blockedChecker.Check(content)
	if len(matched) > 0 {
		return ierr.New(ierr.ErrContentBlocked, "ContentBlocked")
	}
	return nil
}

func (u *anthropicUseCase) checkContent(ctx context.Context, req *dto.AnthropicCreateMessageRequest) error {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractAnthropicMessageText(req)
	matched := u.blockedChecker.Check(content)
	if len(matched) > 0 {
		return ierr.New(ierr.ErrContentBlocked, "ContentBlocked")
	}
	return nil
}
```

- [ ] **Step 4: Add blockedChecker to openAIUseCase struct**

In `openai.go`, add field:
```go
type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    OpenAIProxyPort
	anthropicProxy AnthropicProxyPort
	taskSubmitter  TaskSubmitter
	blockedChecker BlockedChecker
}
```

Update `NewOpenAIUseCase`:
```go
func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
		blockedChecker: blockedChecker,
	}
}
```

Add check at top of `CreateChatCompletion`:
```go
func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	if err := u.checkContent(ctx, req); err != nil {
		return proxyutil.SendOpenAIUpstreamError(req.Body.Model, ierr.ToBizError(err, ierr.ErrInternal.BizError())), nil
	}
	// ... rest unchanged
}
```

- [ ] **Step 5: Add blockedChecker to anthropicUseCase struct**

In `anthropic.go`, add field:
```go
type anthropicUseCase struct {
	resolver         service.EndpointResolver
	modelsQuery      ListAnthropicModels
	countTokensQuery CountTokens
	anthropicProxy   AnthropicProxyPort
	openAIProxy      OpenAIProxyPort
	taskSubmitter    TaskSubmitter
	blockedChecker   BlockedChecker
}
```

Update `NewAnthropicUseCase`:
```go
func NewAnthropicUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListAnthropicModels,
	countTokensQuery CountTokens,
	anthropicProxy AnthropicProxyPort,
	openAIProxy OpenAIProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
) AnthropicUseCase {
	return &anthropicUseCase{
		resolver:         resolver,
		modelsQuery:      modelsQuery,
		countTokensQuery: countTokensQuery,
		anthropicProxy:   anthropicProxy,
		openAIProxy:      openAIProxy,
		taskSubmitter:    taskSubmitter,
		blockedChecker:   blockedChecker,
	}
}
```

Add check at top of `CreateMessage`:
```go
func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	if err := u.checkContent(ctx, req); err != nil {
		return proxyutil.SendAnthropicUpstreamError(req.Body.Model, ierr.ToBizError(err, ierr.ErrInternal.BizError())), nil
	}
	// ... rest unchanged
}
```

- [ ] **Step 6: Update bootstrap wiring**

In `internal/bootstrap/modules/application.go`, update:
```go
func NewCreateChatCompletionHandler(
	resolver service.EndpointResolver,
	modelsQuery port.ListOpenAIModels,
	openAIProxy port.OpenAIProxyPort,
	anthropicProxy port.AnthropicProxyPort,
	taskSubmitter port.TaskSubmitter,
	blockedChecker port.BlockedChecker,
) port.OpenAIUseCase {
	return usecase.NewOpenAIUseCase(resolver, modelsQuery, openAIProxy, anthropicProxy, taskSubmitter, blockedChecker)
}
```

Wait — let me check the actual names in the codebase. The usecase constructor `NewOpenAIUseCase` is already provided. Let me check how it's wired:

In `internal/bootstrap/modules/application.go`:
```go
usecase.NewOpenAIUseCase,
```

So it's directly provided to fx. I just need to add the `BlockedChecker` parameter to the dig module's Provide for `NewOpenAIUseCase`. Since the `openAIUseCase` struct already has `blockedChecker` field, and the constructor already includes the parameter — I need to add `blockedChecker BlockedChecker` as a parameter to the fx.Provide.

But `NewOpenAIUseCase` is directly passed to `fx.Provide`, which means fx infers the parameters from the function signature. So I just need to:
1. Add `blockedChecker BlockedChecker` to the constructor function signature
2. Add a `NewBlockedChecker` provider that returns a `BlockedChecker`

Wait — we need to be more careful. The `NewOpenAIUseCase` is already in `fx.Provide(usecase.NewOpenAIUseCase)`. If I change the function signature, fx will try to inject the new parameter. I just need to make sure there's a provider for `BlockedChecker`.

In the same module, add:
```go
NewBlockedChecker,
```

And:
```go
func NewBlockedChecker(svc *blockedapp.BlockedService) usecase.BlockedChecker {
	return svc
}
```

Wait — the port is defined in the usecase package. Let me check the import. The usecase imports `blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"`. And `BlockedService` is `*blockedapp.BlockedService`. So I need:

```go
import (
	blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
)

func NewBlockedChecker(svc *blockedapp.BlockedService) usecase.BlockedChecker {
	return svc.Check
}
```

Wait — `BlockedChecker` is defined as an interface with `Check(text string) []uint`. But `*blockedapp.BlockedService` also has a `Check(text string) []uint` method. So I can just return `svc.Check` as a function value, but the interface has a `Check` method, so it's: `return svc` (since *BlockedService satisfies BlockedChecker).

Actually, wait. Let me look at the service type more carefully. `BlockedService.Check(text string) []uint`. The `BlockedChecker` interface is:
```go
type BlockedChecker interface {
	Check(text string) []uint
}
```

So `*BlockedService` satisfies `BlockedChecker` because it has `Check(text string) []uint`. So I can just do `return svc` in the provider.

Now, the `Check` method on `BlockedService`:
```go
func (s *BlockedService) Check(text string) []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matcher.Match(text)
}
```

This matches the interface. Good.

So for the DI wiring:
```go
func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy OpenAIProxyPort,
	anthropicProxy AnthropicProxyPort,
	taskSubmitter TaskSubmitter,
	blockedChecker BlockedChecker,
) OpenAIUseCase {
```

And in the application module:
```go
func NewBlockedChecker(svc *blockedapp.BlockedService) usecase.BlockedChecker {
	return svc
}
```

Add this to `ApplicationModule`'s `fx.Provide`.

Also update `NewAnthropicUseCase` the same way.

Actually, looking at the existing code in `internal/bootstrap/modules/application.go`, the usecase constructors are provided directly:
```go
usecase.NewOpenAIUseCase,
usecase.NewAnthropicUseCase,
```

So I need to update the constructor signatures in `openai.go` and `anthropic.go`, and the DI will automatically inject `BlockedChecker` as long as there's a provider for it.

---

### Task 14: Audit for Blocked Requests

**Files:**
- Modify: `internal/application/llmproxy/usecase/blocked_check.go`

When a blocked request is detected, we need to:
1. Return 403 error
2. Create an audit record

- [ ] **Step 1: Update checkContent to also submit audit**

Modify the check logic in `blocked_check.go` to also submit audit task when blocked:

```go
func (u *openAIUseCase) checkContent(ctx context.Context, req *dto.OpenAIChatCompletionRequest) error {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractOpenAIChatText(req)
	matched := u.blockedChecker.Check(content)
	if len(matched) > 0 {
		// Asynchronously audit blocked request
		auditTask := &dto.ModelCallAuditTask{
			Ctx: util.CopyContextValues(ctx),
			ErrorMessage: "trigger blocked word",
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask)
		return ierr.New(ierr.ErrContentBlocked, "ContentBlocked")
	}
	return nil
}
```

Note: for audit we use the existing `ModelCallAuditTask`. The `remark` field will contain "trigger blocked word" as per the spec.

---

### Task 15: Frontend — Types + API Client

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: Add types**

In `web/src/lib/types.ts`:
```typescript
interface CreateBlockedReqBody {
  word: string;
}

interface BlockedItem {
  id: number;
  word: string;
  hitCount: number;
  createdAt: string;
}

interface ListBlockedRsp extends CommonRsp {
  blocked?: BlockedItem[];
  pageInfo?: PageInfo;
}
```

- [ ] **Step 2: Add API methods**

In `web/src/lib/api-client.ts`:
```typescript
async createBlocked(body: CreateBlockedReqBody): Promise<CommonRsp> {
  return this.request<CommonRsp>("/api/v1/block", {
    method: "POST",
    body: JSON.stringify({ body }),
  });
}

async listBlocked(page: number, pageSize: number, query?: string): Promise<ListBlockedRsp> {
  const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (query) params.set("query", query);
  return this.request<ListBlockedRsp>(`/api/v1/block/list?${params}`);
}

async deleteBlocked(id: number): Promise<CommonRsp> {
  return this.request<CommonRsp>(`/api/v1/block/${id}`, { method: "DELETE" });
}
```

---

### Task 16: Frontend — Page Component

**Files:**
- Create: `web/src/app/(dashboard)/block/page.tsx`

- [ ] **Step 1: Create page**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { BlockedItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { Ban, Plus, Search, Trash2, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { Pagination } from "@/components/pagination";

const emptyForm = { word: "" };

export default function BlockPage() {
  const [items, setItems] = useState<BlockedItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.blocked.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.blocked.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<BlockedItem | null>(null);
  const isMobile = useIsMobile();

  const fetchItems = useCallback(async (page: number, pageSize: number, query?: string) => {
    setLoading(true);
    try {
      const safeSize = pageSize > 0 ? pageSize : 20;
      const rsp = await api.listBlocked(page, safeSize, query);
      setItems(rsp.blocked ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch {
      toast.error("Failed to load blocked words");
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

  useEffect(() => { fetchItems(persistedPage, persistedPageSize); }, [fetchItems, persistedPage, persistedPageSize]);

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchItems(1, persistedPageSize, searchQuery || undefined);
  }, [fetchItems, persistedPageSize, searchQuery, setPersistedPage]);

  const handleCreate = useCallback(async () => {
    if (!form.word.trim()) return;
    setSaving(true);
    try {
      await api.createBlocked({ word: form.word.trim() });
      toast.success("Blocked word created");
      setDialogOpen(false);
      setForm(emptyForm);
      fetchItems(persistedPage, persistedPageSize);
    } catch {
      toast.error("Failed to create blocked word");
    } finally {
      setSaving(false);
    }
  }, [form.word, fetchItems, persistedPage, persistedPageSize]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteBlocked(deleteTarget.id);
      toast.success("Blocked word deleted");
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
      fetchItems(persistedPage, persistedPageSize);
    } catch {
      toast.error("Failed to delete blocked word");
    }
  }, [deleteTarget, fetchItems, persistedPage, persistedPageSize]);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">Blocked Words</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">Manage sensitive word blacklist. Words matched in proxy requests will be blocked with 403.</p>
          </div>
          <Button onClick={() => { setForm(emptyForm); setDialogOpen(true); }}>
            <Plus /> Add Word
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">All Blocked Words</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search words..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") handleSearch(); }}
                  className="pl-9"
                />
              </div>
              <Button variant="secondary" onClick={handleSearch}>Search</Button>
            </div>

            {loading ? (
              <div className="space-y-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : items.length === 0 ? (
              <div className="flex flex-col items-center gap-2 py-12 text-muted-foreground">
                <Ban className="size-12" />
                <p>No blocked words yet</p>
              </div>
            ) : isMobile ? (
              <div className="space-y-3">
                {items.map((item) => (
                  <div key={item.id} className="rounded-lg border p-3">
                    <div className="font-medium">{item.word}</div>
                    <div className="mt-1 text-sm text-muted-foreground">
                      Hits: {item.hitCount} · {new Date(item.createdAt).toLocaleDateString()}
                    </div>
                    <Button variant="destructive" size="sm" className="mt-2"
                      onClick={() => { setDeleteTarget(item); setDeleteConfirmOpen(true); }}>
                      <Trash2 /> Delete
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Word</TableHead>
                    <TableHead>Hit Count</TableHead>
                    <TableHead>Created At</TableHead>
                    <TableHead className="w-20">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.id}</TableCell>
                      <TableCell className="font-medium">{item.word}</TableCell>
                      <TableCell>{item.hitCount}</TableCell>
                      <TableCell>{new Date(item.createdAt).toLocaleDateString()}</TableCell>
                      <TableCell>
                        <Button variant="destructive" size="sm"
                          onClick={() => { setDeleteTarget(item); setDeleteConfirmOpen(true); }}>
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination pageInfo={pageInfo} onPageChange={(p) => fetchItems(p, persistedPageSize)} />
          </CardContent>
        </Card>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Add Blocked Word</DialogTitle>
              <DialogDescription>Enter a word to block. Requests containing this word will be rejected.</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <Input
                placeholder="Enter word..."
                value={form.word}
                onChange={(e) => setForm({ word: e.target.value })}
                onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button onClick={handleCreate} disabled={!form.word.trim() || saving}>
                {saving ? "Saving..." : "Create"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <AlertTriangle className="size-5 text-destructive" /> Are you sure?
              </AlertDialogTitle>
              <AlertDialogDescription>
                Delete blocked word "{deleteTarget?.word}"? This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={handleDelete}>Delete</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </PermissionGuard>
  );
}
```

---

### Task 17: Frontend — Sidebar Nav

**Files:**
- Modify: `web/src/app/(dashboard)/layout.tsx`

- [ ] **Step 1: Add nav item**

Import `Ban` icon:
```typescript
import { Ban, Cpu, Key, LayoutDashboard, MessageSquare, ScrollText, Server, Share2, User } from "lucide-react";
```

Add to `navItems` array (after "Models", before "Audit"):
```typescript
{ label: "Blocked",  href: "/block/",      icon: <Ban />,         adminOnly: true },
```

---

### Verifications

After all tasks are complete:
- Run `make build` — should compile cleanly
- Run `make lint` — should pass
- Run `go test -count=1 ./...` — all tests pass
- Run `cd web && npm run lint && npm run build` — frontend builds
