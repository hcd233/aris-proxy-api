package message_repository

import (
	"context"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestUpdateMessageContentSerializesUnifiedMessage(t *testing.T) {
	t.Parallel()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Message{}); err != nil {
		t.Fatalf("failed to migrate sqlite db: %v", err)
	}

	stored := &dbmodel.Message{
		ModelID: "test-model",
		Message: &vo.UnifiedMessage{
			Role:    enum.RoleAssistant,
			Content: &vo.UnifiedContent{Text: "before"},
		},
		CheckSum: "test-checksum",
	}
	if err := db.Create(stored).Error; err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	updated := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "after"},
		ReasoningContent: "thinking",
		ToolCalls: []*vo.UnifiedToolCall{{
			ID:        "call-1",
			Name:      "read",
			Arguments: `{"path":"README.md"}`,
		}},
	}
	repo := repository.NewThinkExtractRepository(db)
	if err := repo.UpdateMessageContent(context.Background(), stored.ID, updated); err != nil {
		t.Fatalf("UpdateMessageContent returned error: %v", err)
	}

	var raw string
	if err := db.Raw("SELECT message FROM messages WHERE id = ?", stored.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("failed to read raw message: %v", err)
	}
	if !sonic.Valid([]byte(raw)) {
		t.Fatalf("message column is not valid JSON: %q", raw)
	}

	var reloaded dbmodel.Message
	if err := db.First(&reloaded, stored.ID).Error; err != nil {
		t.Fatalf("failed to reload message: %v", err)
	}
	if reloaded.Message == nil {
		t.Fatal("reloaded message is nil")
	}
	if reloaded.Message.ReasoningContent != updated.ReasoningContent {
		t.Fatalf("reasoning content = %q, want %q", reloaded.Message.ReasoningContent, updated.ReasoningContent)
	}
	if reloaded.Message.Content == nil || reloaded.Message.Content.Text != updated.Content.Text {
		t.Fatalf("content = %#v, want %q", reloaded.Message.Content, updated.Content.Text)
	}
	if len(reloaded.Message.ToolCalls) != 1 || reloaded.Message.ToolCalls[0].Arguments != updated.ToolCalls[0].Arguments {
		t.Fatalf("tool calls = %#v, want %#v", reloaded.Message.ToolCalls, updated.ToolCalls)
	}
}
