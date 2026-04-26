package session_service

import (
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/service"
)

// messageFixture represents a message entry in the fixture JSON
type messageFixture struct {
	ID        uint                `json:"id"`
	Model     string              `json:"model"`
	Message   *dto.UnifiedMessage `json:"message"`
	CreatedAt string              `json:"created_at"`
}

// toolFixture represents a tool entry in the fixture JSON
type toolFixture struct {
	ID        uint             `json:"id"`
	Tool      *dto.UnifiedTool `json:"tool"`
	CreatedAt string           `json:"created_at"`
}

// sessionServiceCase represents a test case loaded from fixtures/cases.json
type sessionServiceCase struct {
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	MessageIDs           []uint            `json:"message_ids,omitempty"`
	Messages             []*messageFixture `json:"messages,omitempty"`
	ExpectedIDs          []uint            `json:"expected_ids,omitempty"`
	ExpectedCount        int               `json:"expected_count"`
	ExpectedModel        string            `json:"expected_model,omitempty"`
	ExpectedRole         string            `json:"expected_role,omitempty"`
	ExpectedCreatedAt    string            `json:"expected_created_at,omitempty"`
	ToolIDs              []uint            `json:"tool_ids,omitempty"`
	Tools                []*toolFixture    `json:"tools,omitempty"`
	ExpectedToolIDs      []uint            `json:"expected_tool_ids,omitempty"`
	ExpectedToolNames    []string          `json:"expected_tool_names,omitempty"`
	ExpectedToolCount    int               `json:"expected_tool_count"`
	ExpectedToolCreateAt string            `json:"expected_tool_created_at,omitempty"`
}

// loadCases loads test cases from fixtures/cases.json
func loadCases(t *testing.T) []sessionServiceCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []sessionServiceCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// findCase finds a test case by name, fatals if not found
func findCase(t *testing.T, cases []sessionServiceCase, name string) sessionServiceCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return sessionServiceCase{}
}

// toDBMessages converts fixture messages to database model messages
func toDBMessages(t *testing.T, fixtures []*messageFixture) []*dbmodel.Message {
	t.Helper()
	messages := make([]*dbmodel.Message, len(fixtures))
	for i, f := range fixtures {
		createdAt, err := time.Parse(time.RFC3339, f.CreatedAt)
		if err != nil {
			t.Fatalf("failed to parse created_at %q for message %d: %v", f.CreatedAt, f.ID, err)
		}
		messages[i] = &dbmodel.Message{
			ID:        f.ID,
			Model:     f.Model,
			Message:   f.Message,
			BaseModel: dbmodel.BaseModel{CreatedAt: createdAt},
		}
	}
	return messages
}

// toDBTools converts fixture tools to database model tools
func toDBTools(t *testing.T, fixtures []*toolFixture) []*dbmodel.Tool {
	t.Helper()
	tools := make([]*dbmodel.Tool, len(fixtures))
	for i, f := range fixtures {
		createdAt, err := time.Parse(time.RFC3339, f.CreatedAt)
		if err != nil {
			t.Fatalf("failed to parse created_at %q for tool %d: %v", f.CreatedAt, f.ID, err)
		}
		tools[i] = &dbmodel.Tool{
			ID:        f.ID,
			Tool:      f.Tool,
			BaseModel: dbmodel.BaseModel{CreatedAt: createdAt},
		}
	}
	return tools
}

func TestBuildOrderedMessages(t *testing.T) {
	allCases := loadCases(t)

	messageCases := []string{
		"build_ordered_messages_preserves_order",
		"build_ordered_messages_skips_missing_ids",
		"build_ordered_messages_empty_ids",
		"build_ordered_messages_populates_fields",
	}

	for _, caseName := range messageCases {
		tc := findCase(t, allCases, caseName)
		t.Run(tc.Description, func(t *testing.T) {
			messages := toDBMessages(t, tc.Messages)
			items := service.BuildOrderedMessages(tc.MessageIDs, messages)

			if len(items) != tc.ExpectedCount {
				t.Fatalf("BuildOrderedMessages() returned %d items, want %d", len(items), tc.ExpectedCount)
			}

			for i, item := range items {
				if i < len(tc.ExpectedIDs) && item.ID != tc.ExpectedIDs[i] {
					t.Errorf("items[%d].ID = %d, want %d", i, item.ID, tc.ExpectedIDs[i])
				}
			}

			// verify detailed fields for populates_fields case
			if tc.ExpectedModel != "" && len(items) > 0 {
				if items[0].Model != tc.ExpectedModel {
					t.Errorf("Model = %q, want %q", items[0].Model, tc.ExpectedModel)
				}
			}
			if tc.ExpectedRole != "" && len(items) > 0 {
				if items[0].Message.Role != tc.ExpectedRole {
					t.Errorf("Message.Role = %q, want %q", items[0].Message.Role, tc.ExpectedRole)
				}
			}
			if tc.ExpectedCreatedAt != "" && len(items) > 0 {
				if items[0].CreatedAt != tc.ExpectedCreatedAt {
					t.Errorf("CreatedAt = %q, want %q", items[0].CreatedAt, tc.ExpectedCreatedAt)
				}
			}
		})
	}
}

func TestBuildOrderedMessages_NilInputs(t *testing.T) {
	items := service.BuildOrderedMessages(nil, nil)
	if len(items) != 0 {
		t.Errorf("BuildOrderedMessages(nil, nil) returned %d items, want 0", len(items))
	}
}

func TestBuildOrderedTools(t *testing.T) {
	allCases := loadCases(t)

	toolCases := []string{
		"build_ordered_tools_preserves_order",
		"build_ordered_tools_skips_missing_ids",
		"build_ordered_tools_empty_ids",
		"build_ordered_tools_populates_fields",
	}

	for _, caseName := range toolCases {
		tc := findCase(t, allCases, caseName)
		t.Run(tc.Description, func(t *testing.T) {
			tools := toDBTools(t, tc.Tools)
			items := service.BuildOrderedTools(tc.ToolIDs, tools)

			if len(items) != tc.ExpectedToolCount {
				t.Fatalf("BuildOrderedTools() returned %d items, want %d", len(items), tc.ExpectedToolCount)
			}

			for i, item := range items {
				if i < len(tc.ExpectedToolIDs) && item.ID != tc.ExpectedToolIDs[i] {
					t.Errorf("items[%d].ID = %d, want %d", i, item.ID, tc.ExpectedToolIDs[i])
				}
				if i < len(tc.ExpectedToolNames) && item.Tool.Name != tc.ExpectedToolNames[i] {
					t.Errorf("items[%d].Tool.Name = %q, want %q", i, item.Tool.Name, tc.ExpectedToolNames[i])
				}
			}

			// verify detailed fields for populates_fields case
			if tc.ExpectedToolCreateAt != "" && len(items) > 0 {
				if items[0].CreatedAt != tc.ExpectedToolCreateAt {
					t.Errorf("CreatedAt = %q, want %q", items[0].CreatedAt, tc.ExpectedToolCreateAt)
				}
			}
		})
	}
}

func TestBuildOrderedTools_NilInputs(t *testing.T) {
	items := service.BuildOrderedTools(nil, nil)
	if len(items) != 0 {
		t.Errorf("BuildOrderedTools(nil, nil) returned %d items, want 0", len(items))
	}
}
