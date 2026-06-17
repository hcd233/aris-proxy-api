package cron_test

import (
	"context"
	"testing"
	"time"

	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronauditquery "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/query"
	cronmgmtcommand "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/command"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	cronmgmtquery "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/query"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

type fakeCronJobRepo struct {
	jobs []*cronmgmtport.CronJobView
	err  error
}

func (r *fakeCronJobRepo) Sync(ctx context.Context, jobs []*cronmgmtport.CronJobView) error {
	return r.err
}

func (r *fakeCronJobRepo) List(ctx context.Context, param dao.CommonParam) ([]*cronmgmtport.CronJobView, *model.PageInfo, error) {
	if r.err != nil {
		return nil, nil, r.err
	}
	return r.jobs, &model.PageInfo{Page: 1, PageSize: 20, Total: int64(len(r.jobs))}, nil
}

func (r *fakeCronJobRepo) Update(ctx context.Context, name string, params cronmgmtport.UpdateCronJobParams) error {
	if r.err != nil {
		return r.err
	}
	for _, job := range r.jobs {
		if job.Name == name {
			if params.Enabled != nil {
				job.Enabled = *params.Enabled
			}
			if params.Spec != nil {
				job.Spec = *params.Spec
			}
			return nil
		}
	}
	return ierr.New(ierr.ErrDataNotExists, "cron job not found")
}

func (r *fakeCronJobRepo) Get(ctx context.Context, name string) (*cronmgmtport.CronJobView, error) {
	for _, job := range r.jobs {
		if job.Name == name {
			return job, nil
		}
	}
	return nil, ierr.New(ierr.ErrDataNotExists, "cron job not found")
}

type fakeCronCallAuditRepo struct {
	logs []*cronauditport.CronCallAuditView
	err  error
}

func (r *fakeCronCallAuditRepo) Save(ctx context.Context, audit *cronauditport.CronCallAuditView) error {
	if r.err != nil {
		return r.err
	}
	r.logs = append(r.logs, audit)
	return nil
}

func (r *fakeCronCallAuditRepo) List(ctx context.Context, param dao.CommonParam, startTime, endTime time.Time, filterExp string) ([]*cronauditport.CronCallAuditView, *model.PageInfo, error) {
	if r.err != nil {
		return nil, nil, r.err
	}
	return r.logs, &model.PageInfo{Page: 1, PageSize: 20, Total: int64(len(r.logs))}, nil
}

func (r *fakeCronCallAuditRepo) ListDistinctTypes(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []string{"SessionDeduplicateCron"}, nil
}

func (r *fakeCronCallAuditRepo) ListDistinctStatuses(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []string{"success", "failed", "panic", "skipped"}, nil
}

func newCronHandlerForTest() (handler.CronHandler, *fakeCronJobRepo, *fakeCronCallAuditRepo) {
	jobRepo := &fakeCronJobRepo{
		jobs: []*cronmgmtport.CronJobView{
			{Name: "SessionDeduplicateCron", Type: "functional", Spec: "0 * * * *", Description: "dedup", Enabled: true},
		},
	}
	auditRepo := &fakeCronCallAuditRepo{}
	return handler.NewCronHandler(handler.CronDependencies{
		ListCronJobs:             cronmgmtquery.NewListCronJobsHandler(jobRepo),
		UpdateCronJob:            cronmgmtcommand.NewUpdateCronJobHandler(jobRepo, nil),
		ListCronCallAudits:       cronauditquery.NewListCronCallAuditsHandler(auditRepo),
		ListCronCallAuditOptions: cronauditquery.NewListCronCallAuditOptionsHandler(auditRepo),
	}), jobRepo, auditRepo
}

func TestCronHandler_ListCronJobs_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := newCronHandlerForTest()
	rsp, err := h.HandleListCronJobs(context.Background(), &dto.ListCronJobsReq{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Fatalf("unexpected business error: %v", rsp.Body.Error)
	}
	if len(rsp.Body.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(rsp.Body.Jobs))
	}
	if rsp.Body.Jobs[0].Name != "SessionDeduplicateCron" {
		t.Errorf("unexpected job name: %s", rsp.Body.Jobs[0].Name)
	}
}

func TestCronHandler_UpdateCronJob_Success(t *testing.T) {
	t.Parallel()
	h, jobRepo, _ := newCronHandlerForTest()
	disabled := false
	rsp, err := h.HandleUpdateCronJob(context.Background(), &dto.UpdateCronJobReq{
		Name: "SessionDeduplicateCron",
		Body: &dto.UpdateCronJobReqBody{Enabled: &disabled},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Fatalf("unexpected business error: %v", rsp.Body.Error)
	}
	if jobRepo.jobs[0].Enabled {
		t.Error("expected job to be disabled")
	}
}

func TestCronHandler_UpdateCronJob_MissingBody(t *testing.T) {
	t.Parallel()
	h, _, _ := newCronHandlerForTest()
	rsp, err := h.HandleUpdateCronJob(context.Background(), &dto.UpdateCronJobReq{Name: "SessionDeduplicateCron"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error == nil {
		t.Fatal("expected validation error")
	}
}

func TestCronHandler_ListCronCallAudits_Success(t *testing.T) {
	t.Parallel()
	h, _, auditRepo := newCronHandlerForTest()
	auditRepo.logs = []*cronauditport.CronCallAuditView{
		{ID: 1, CronName: "SessionDeduplicateCron", Status: "success", DurationMs: 100},
	}
	rsp, err := h.HandleListCronCallAudits(context.Background(), &dto.ListCronCallAuditsReq{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Fatalf("unexpected business error: %v", rsp.Body.Error)
	}
	if len(rsp.Body.Logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(rsp.Body.Logs))
	}
	if rsp.Body.Logs[0].CronName != "SessionDeduplicateCron" {
		t.Errorf("unexpected cron name: %s", rsp.Body.Logs[0].CronName)
	}
}

func TestCronHandler_ListCronCallAuditOptions_TypeField(t *testing.T) {
	t.Parallel()
	h, _, _ := newCronHandlerForTest()
	rsp, err := h.HandleListCronCallAuditOptions(context.Background(), &dto.CronCallAuditOptionListReq{Field: "type"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Fatalf("unexpected business error: %v", rsp.Body.Error)
	}
	if len(rsp.Body.Items) != 1 {
		t.Fatalf("expected 1 option, got %d", len(rsp.Body.Items))
	}
	if rsp.Body.Items[0] != "SessionDeduplicateCron" {
		t.Errorf("unexpected type option: %s", rsp.Body.Items[0])
	}
}

func TestCronHandler_ListCronCallAuditOptions_StatusField(t *testing.T) {
	t.Parallel()
	h, _, _ := newCronHandlerForTest()
	rsp, err := h.HandleListCronCallAuditOptions(context.Background(), &dto.CronCallAuditOptionListReq{Field: "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp.Body.Error != nil {
		t.Fatalf("unexpected business error: %v", rsp.Body.Error)
	}
	if len(rsp.Body.Items) != 4 {
		t.Fatalf("expected 4 status options, got %d", len(rsp.Body.Items))
	}
}
