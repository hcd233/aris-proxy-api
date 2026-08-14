package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
)

type Trigger struct {
	commonagg.Base
	word      string
	action    string
	hitCount  uint
	createdAt time.Time
	updatedAt time.Time
}

func CreateTrigger(id uint, word, action string) (*Trigger, error) {
	if word == "" {
		return nil, ierr.New(ierr.ErrValidation, "trigger word cannot be empty")
	}
	if action == "" {
		action = enum.TriggerActionDeny
	}
	b := &Trigger{word: word, action: action}
	b.SetID(id)
	return b, nil
}

func (b *Trigger) Word() string              { return b.word }
func (b *Trigger) Action() string            { return b.action }
func (b *Trigger) HitCount() uint            { return b.hitCount }
func (b *Trigger) CreatedAt() time.Time      { return b.createdAt }
func (b *Trigger) UpdatedAt() time.Time      { return b.updatedAt }
func (b *Trigger) SetHitCount(hitCount uint) { b.hitCount = hitCount }
func (b *Trigger) SetTimestamps(createdAt, updatedAt time.Time) {
	b.createdAt = createdAt
	b.updatedAt = updatedAt
}
