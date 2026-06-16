package message_dedup

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Message{}, &dbmodel.Tool{}, &dbmodel.Session{}); err != nil {
		t.Fatalf("failed to migrate sqlite db: %v", err)
	}
	return db
}

func equivalentReasoningMessages() []*vo.UnifiedMessage {
	return []*vo.UnifiedMessage{
		{
			Role:    enum.RoleUser,
			Content: &vo.UnifiedContent{Text: "same reasoning"},
		},
		{
			Role:             enum.RoleUser,
			ReasoningContent: "same reasoning",
		},
	}
}

func equivalentToolCallMessages() []*vo.UnifiedMessage {
	return []*vo.UnifiedMessage{
		{
			Role: enum.RoleAssistant,
			ToolCalls: []*vo.UnifiedToolCall{
				{ID: "call_001", Name: "read", Arguments: `{"path":"README.md"}`},
			},
		},
		{
			Role: enum.RoleAssistant,
			ToolCalls: []*vo.UnifiedToolCall{
				{ID: "call_999", Name: "read", Arguments: `{"path":"README.md"}`},
			},
		},
	}
}

func TestMessageRepositoryBatchSaveDedup_DeduplicatesEquivalentMessagesInsideBatch(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewMessageRepository(db)
	unifiedMessages := equivalentReasoningMessages()
	checksum := vo.ComputeMessageChecksum(unifiedMessages[0], "", nil)
	if checksum != vo.ComputeMessageChecksum(unifiedMessages[1], "", nil) {
		t.Fatal("test setup expected content/reasoning messages to share checksum")
	}

	messages := make([]*aggregate.Message, 0, len(unifiedMessages))
	for _, msg := range unifiedMessages {
		record, err := aggregate.RecordMessage(msg, "", checksum)
		if err != nil {
			t.Fatalf("RecordMessage returned error: %v", err)
		}
		messages = append(messages, record)
	}

	ids, err := repo.BatchSaveDedup(context.Background(), messages)
	if err != nil {
		t.Fatalf("BatchSaveDedup returned error: %v", err)
	}
	assertDeduplicatedMessages(t, db, ids)
}

func TestMessageStoreTask_DeduplicatesEquivalentMessagesInsideBatch(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	pm := pool.NewPoolManager(db)
	task := &dto.MessageStoreTask{
		Ctx:        context.Background(),
		APIKeyName: "test-key",
		Model:      "deepseek-v4-pro",
		Messages:   equivalentToolCallMessages(),
		Metadata:   map[string]string{},
	}

	if err := pm.SubmitMessageStoreTask(task); err != nil {
		t.Fatalf("SubmitMessageStoreTask returned error: %v", err)
	}
	ids := waitForSessionMessageIDs(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pm.StopWithContext(ctx); err != nil {
		t.Fatalf("StopWithContext returned error: %v", err)
	}
	assertDeduplicatedMessages(t, db, ids)
}

func waitForSessionMessageIDs(t *testing.T, db *gorm.DB) []uint {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var session dbmodel.Session
		err := db.First(&session).Error
		if err == nil {
			return session.MessageIDs
		}
		select {
		case <-ctx.Done():
			t.Fatalf("session was not stored before timeout: %v", err)
		case <-ticker.C:
		}
	}
}

func assertDeduplicatedMessages(t *testing.T, db *gorm.DB, ids []uint) {
	t.Helper()
	if len(ids) != 2 {
		t.Fatalf("id count = %d, want 2", len(ids))
	}
	if ids[0] == 0 || ids[0] != ids[1] {
		t.Fatalf("ids = %v, want two references to the same stored message", ids)
	}

	var count int64
	if err := db.Model(&dbmodel.Message{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored message count = %d, want 1", count)
	}
}
