package port

import "context"

type HitRecorder interface {
	IncrementHits(ctx context.Context, ids []uint) error
}
