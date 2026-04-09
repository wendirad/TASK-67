package services

import (
	"time"

	"campusrec/internal/repository"
)

type KPIService struct {
	kpiRepo *repository.KPIRepository
}

func NewKPIService(kpiRepo *repository.KPIRepository) *KPIService {
	return &KPIService{kpiRepo: kpiRepo}
}

func (s *KPIService) Overview(from, to time.Time, facility string) (*repository.OverviewResult, error) {
	return s.kpiRepo.Overview(from, to, facility)
}

func (s *KPIService) FillRate(from, to time.Time, facility, granularity string) ([]repository.FillRateTimePoint, error) {
	return s.kpiRepo.FillRateTimeSeries(from, to, facility, granularity)
}

func (s *KPIService) Members(from, to time.Time, granularity string) ([]repository.MemberTimePoint, error) {
	return s.kpiRepo.MemberMetrics(from, to, granularity)
}

func (s *KPIService) Engagement(from, to time.Time) (*repository.EngagementResult, error) {
	return s.kpiRepo.EngagementMetrics(from, to)
}

func (s *KPIService) Coaches(from, to time.Time) ([]repository.CoachResult, error) {
	return s.kpiRepo.CoachProductivity(from, to)
}

func (s *KPIService) Revenue(from, to time.Time, granularity string) (*repository.RevenueSummary, error) {
	return s.kpiRepo.RevenueMetrics(from, to, granularity)
}

func (s *KPIService) Tickets() (*repository.TicketMetricsResult, error) {
	return s.kpiRepo.TicketMetrics()
}
