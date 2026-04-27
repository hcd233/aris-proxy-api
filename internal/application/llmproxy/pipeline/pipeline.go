package pipeline

import "context"

type Step[T any] interface {
	Name() string
	Run(ctx context.Context, state *T) error
}

type StepFunc[T any] struct {
	name string
	fn   func(ctx context.Context, state *T) error
}

func NewStep[T any](name string, fn func(ctx context.Context, state *T) error) Step[T] {
	return &StepFunc[T]{name: name, fn: fn}
}

func (s *StepFunc[T]) Name() string {
	return s.name
}

func (s *StepFunc[T]) Run(ctx context.Context, state *T) error {
	if s.fn == nil {
		return nil
	}
	return s.fn(ctx, state)
}

type Pipeline[T any] struct {
	steps []Step[T]
}

func NewPipeline[T any](steps ...Step[T]) *Pipeline[T] {
	return &Pipeline[T]{steps: steps}
}

func (p *Pipeline[T]) Execute(ctx context.Context, state *T) error {
	for _, step := range p.steps {
		if err := step.Run(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline[T]) StepNames() []string {
	names := make([]string, 0, len(p.steps))
	for _, step := range p.steps {
		names = append(names, step.Name())
	}
	return names
}
