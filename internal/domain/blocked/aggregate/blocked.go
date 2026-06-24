package aggregate

import (
	"time"

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

func (b *Blocked) Word() string              { return b.word }
func (b *Blocked) HitCount() uint            { return b.hitCount }
func (b *Blocked) CreatedAt() time.Time      { return b.createdAt }
func (b *Blocked) UpdatedAt() time.Time      { return b.updatedAt }
func (b *Blocked) SetHitCount(hitCount uint) { b.hitCount = hitCount }
func (b *Blocked) SetTimestamps(createdAt, updatedAt time.Time) {
	b.createdAt = createdAt
	b.updatedAt = updatedAt
}
