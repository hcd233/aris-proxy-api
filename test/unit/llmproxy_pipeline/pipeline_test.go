package llmproxy_pipeline

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline"
)

type testState struct {
	calls []string
}

func TestPipelineExecute_RunsStepsInOrder(t *testing.T) {
	state := &testState{}
	p := pipeline.NewPipeline(
		pipeline.NewStep("first", func(_ context.Context, s *testState) error {
			s.calls = append(s.calls, "first")
			return nil
		}),
		pipeline.NewStep("second", func(_ context.Context, s *testState) error {
			s.calls = append(s.calls, "second")
			return nil
		}),
	)

	if err := p.Execute(context.Background(), state); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	want := []string{"first", "second"}
	if !reflect.DeepEqual(state.calls, want) {
		t.Fatalf("calls = %v, want %v", state.calls, want)
	}
}

func TestPipelineExecute_StopsOnError(t *testing.T) {
	state := &testState{}
	expectedErr := errors.New("stop")
	p := pipeline.NewPipeline(
		pipeline.NewStep("first", func(_ context.Context, s *testState) error {
			s.calls = append(s.calls, "first")
			return expectedErr
		}),
		pipeline.NewStep("second", func(_ context.Context, s *testState) error {
			s.calls = append(s.calls, "second")
			return nil
		}),
	)

	err := p.Execute(context.Background(), state)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Execute() error = %v, want %v", err, expectedErr)
	}

	want := []string{"first"}
	if !reflect.DeepEqual(state.calls, want) {
		t.Fatalf("calls = %v, want %v", state.calls, want)
	}
}

func TestPipelineStepNames(t *testing.T) {
	p := pipeline.NewPipeline(
		pipeline.NewStep("resolve_endpoint", func(_ context.Context, _ *testState) error { return nil }),
		pipeline.NewStep("select_route", func(_ context.Context, _ *testState) error { return nil }),
	)

	want := []string{"resolve_endpoint", "select_route"}
	if !reflect.DeepEqual(p.StepNames(), want) {
		t.Fatalf("StepNames() = %v, want %v", p.StepNames(), want)
	}
}
