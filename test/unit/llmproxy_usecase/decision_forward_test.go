package llmproxy_usecase

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestMarshalOpenAIDecisionBodyForModel_RewritesModelOnly(t *testing.T) {
	t.Parallel()

	name := "damaged"
	req := &dto.OpenAICreateDecisionReq{
		Model: "exposed-alias",
		Input: dto.DecisionInput{Text: lo.ToPtr("The package arrived with a broken screen.")},
		Questions: []*dto.DecisionQuestion{{
			Type:         "predicate",
			Name:         &name,
			Instructions: "Does the customer report a damaged item?",
		}},
	}

	body := proxyutil.MarshalOpenAIDecisionBodyForModel(req, "gpt-upstream-real")

	var got map[string]any
	if err := sonic.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["model"] != "gpt-upstream-real" {
		t.Fatalf("model = %v, want gpt-upstream-real", got["model"])
	}
	if got["input"] != "The package arrived with a broken screen." {
		t.Fatalf("input = %v, want the original text", got["input"])
	}
	questions, ok := got["questions"].([]any)
	if !ok || len(questions) != 1 {
		t.Fatalf("questions = %v, want 1 item", got["questions"])
	}
	if req.Model != "exposed-alias" {
		t.Fatalf("original request mutated: model = %s", req.Model)
	}
}
