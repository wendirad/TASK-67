package repository

import (
	"database/sql"
	"fmt"
	"time"
)

type KPIRepository struct {
	db *sql.DB
}

func NewKPIRepository(db *sql.DB) *KPIRepository {
	return &KPIRepository{db: db}
}

// FillRateByFacility returns average fill rate per facility in a date range.
type FillRateResult struct {
	FacilityName string  `json:"facility_name"`
	FillRate     float64 `json:"fill_rate"`
	SessionCount int     `json:"session_count"`
}

func (r *KPIRepository) FillRateByFacility(from, to time.Time, facility string) ([]FillRateResult, error) {
	query := `
		SELECT f.name AS facility_name,
		    AVG(
		        (SELECT COUNT(*)::float FROM registrations reg
		         WHERE reg.session_id = s.id
		           AND reg.status IN ('registered', 'completed', 'no_show'))
		        / NULLIF(s.total_seats, 0) * 100
		    ) as fill_rate,
		    COUNT(*) as session_count
		FROM sessions s
		JOIN facilities f ON s.facility_id = f.id
		WHERE s.status IN ('completed', 'in_progress')
		    AND s.start_time BETWEEN $1 AND $2`
	args := []interface{}{from, to}
	argIdx := 3

	if facility != "" {
		query += fmt.Sprintf(` AND f.name = $%d`, argIdx)
		args = append(args, facility)
	}

	query += ` GROUP BY f.name ORDER BY f.name`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fill rate query: %w", err)
	}
	defer rows.Close()

	var results []FillRateResult
	for rows.Next() {
		var fr FillRateResult
		var rate *float64
		if err := rows.Scan(&fr.FacilityName, &rate, &fr.SessionCount); err != nil {
			return nil, fmt.Errorf("scan fill rate: %w", err)
		}
		if rate != nil {
			fr.FillRate = *rate
		}
		results = append(results, fr)
	}
	return results, rows.Err()
}

// FillRateTimeSeries returns fill rate data over time.
type FillRateTimePoint struct {
	Period       string  `json:"period"`
	FillRate     float64 `json:"fill_rate"`
	SessionCount int     `json:"session_count"`
}

func (r *KPIRepository) FillRateTimeSeries(from, to time.Time, facility, granularity string) ([]FillRateTimePoint, error) {
	trunc := granularityToTrunc(granularity)
	query := fmt.Sprintf(`
		SELECT DATE_TRUNC('%s', s.start_time)::date as period,
		    AVG(
		        (SELECT COUNT(*)::float FROM registrations reg
		         WHERE reg.session_id = s.id
		           AND reg.status IN ('registered', 'completed', 'no_show'))
		        / NULLIF(s.total_seats, 0) * 100
		    ) as fill_rate,
		    COUNT(*) as session_count
		FROM sessions s
		JOIN facilities f ON s.facility_id = f.id
		WHERE s.status IN ('completed', 'in_progress')
		    AND s.start_time BETWEEN $1 AND $2`, trunc)
	args := []interface{}{from, to}
	argIdx := 3

	if facility != "" {
		query += fmt.Sprintf(` AND f.name = $%d`, argIdx)
		args = append(args, facility)
	}

	query += fmt.Sprintf(` GROUP BY DATE_TRUNC('%s', s.start_time)::date ORDER BY period`, trunc)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fill rate time series: %w", err)
	}
	defer rows.Close()

	var results []FillRateTimePoint
	for rows.Next() {
		var fp FillRateTimePoint
		var rate *float64
		var period time.Time
		if err := rows.Scan(&period, &rate, &fp.SessionCount); err != nil {
			return nil, fmt.Errorf("scan fill rate time: %w", err)
		}
		fp.Period = period.Format("2006-01-02")
		if rate != nil {
			fp.FillRate = *rate
		}
		results = append(results, fp)
	}
	return results, rows.Err()
}

// MemberMetrics returns growth and churn data.
type MemberTimePoint struct {
	Period  string `json:"period"`
	Growth  int    `json:"growth"`
	Churn   int    `json:"churn"`
	NetGain int    `json:"net_gain"`
}

func (r *KPIRepository) MemberMetrics(from, to time.Time, granularity string) ([]MemberTimePoint, error) {
	trunc := granularityToTrunc(granularity)

	// Growth: new member accounts created
	growthQuery := fmt.Sprintf(`
		SELECT DATE_TRUNC('%s', created_at)::date as period, COUNT(*) as cnt
		FROM users
		WHERE role = 'member' AND created_at BETWEEN $1 AND $2
		GROUP BY period ORDER BY period
	`, trunc)

	growthRows, err := r.db.Query(growthQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("growth query: %w", err)
	}
	defer growthRows.Close()

	growthMap := map[string]int{}
	for growthRows.Next() {
		var period time.Time
		var cnt int
		if err := growthRows.Scan(&period, &cnt); err != nil {
			return nil, fmt.Errorf("scan growth: %w", err)
		}
		growthMap[period.Format("2006-01-02")] = cnt
	}

	// Churn: members who became inactive or banned
	churnQuery := fmt.Sprintf(`
		SELECT DATE_TRUNC('%s', updated_at)::date as period, COUNT(*) as cnt
		FROM users
		WHERE role = 'member' AND status IN ('inactive', 'banned')
		    AND updated_at BETWEEN $1 AND $2
		GROUP BY period ORDER BY period
	`, trunc)

	churnRows, err := r.db.Query(churnQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("churn query: %w", err)
	}
	defer churnRows.Close()

	churnMap := map[string]int{}
	for churnRows.Next() {
		var period time.Time
		var cnt int
		if err := churnRows.Scan(&period, &cnt); err != nil {
			return nil, fmt.Errorf("scan churn: %w", err)
		}
		churnMap[period.Format("2006-01-02")] = cnt
	}

	// Merge into combined results
	allPeriods := map[string]bool{}
	for k := range growthMap {
		allPeriods[k] = true
	}
	for k := range churnMap {
		allPeriods[k] = true
	}

	var results []MemberTimePoint
	for p := range allPeriods {
		g := growthMap[p]
		c := churnMap[p]
		results = append(results, MemberTimePoint{
			Period: p, Growth: g, Churn: c, NetGain: g - c,
		})
	}

	// Sort by period
	sortMemberTimePoints(results)
	return results, nil
}

// EngagementMetrics returns active member and avg session counts.
type EngagementResult struct {
	ActiveMembers     int     `json:"active_members"`
	TotalCheckIns     int     `json:"total_check_ins"`
	AvgSessionsPerMember float64 `json:"avg_sessions_per_member"`
	TotalOrders       int     `json:"total_orders"`
}

func (r *KPIRepository) EngagementMetrics(from, to time.Time) (*EngagementResult, error) {
	result := &EngagementResult{}

	// Check-in based engagement
	err := r.db.QueryRow(`
		SELECT COALESCE(COUNT(DISTINCT user_id), 0),
		       COALESCE(COUNT(*), 0),
		       COALESCE(COUNT(*)::float / NULLIF(COUNT(DISTINCT user_id), 0), 0)
		FROM check_ins
		WHERE checked_in_at BETWEEN $1 AND $2
	`, from, to).Scan(&result.ActiveMembers, &result.TotalCheckIns, &result.AvgSessionsPerMember)
	if err != nil {
		return nil, fmt.Errorf("engagement check-in query: %w", err)
	}

	// Order count
	err = r.db.QueryRow(`
		SELECT COALESCE(COUNT(*), 0) FROM orders
		WHERE created_at BETWEEN $1 AND $2 AND status NOT IN ('closed', 'canceled')
	`, from, to).Scan(&result.TotalOrders)
	if err != nil {
		return nil, fmt.Errorf("engagement order query: %w", err)
	}

	return result, nil
}

// CoachProductivity returns session count and fill rate per coach.
type CoachResult struct {
	CoachName    string  `json:"coach_name"`
	SessionCount int     `json:"session_count"`
	AvgFillRate  float64 `json:"avg_fill_rate"`
}

func (r *KPIRepository) CoachProductivity(from, to time.Time) ([]CoachResult, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(s.coach_name, 'Unassigned') as coach_name,
		    COUNT(*) as session_count,
		    AVG(
		        (SELECT COUNT(*)::float FROM registrations reg
		         WHERE reg.session_id = s.id
		           AND reg.status IN ('registered', 'completed', 'no_show'))
		        / NULLIF(s.total_seats, 0) * 100
		    ) as avg_fill_rate
		FROM sessions s
		WHERE s.status = 'completed' AND s.start_time BETWEEN $1 AND $2
		GROUP BY s.coach_name
		ORDER BY session_count DESC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("coach productivity: %w", err)
	}
	defer rows.Close()

	var results []CoachResult
	for rows.Next() {
		var cr CoachResult
		var rate *float64
		if err := rows.Scan(&cr.CoachName, &cr.SessionCount, &rate); err != nil {
			return nil, fmt.Errorf("scan coach: %w", err)
		}
		if rate != nil {
			cr.AvgFillRate = *rate
		}
		results = append(results, cr)
	}
	return results, rows.Err()
}

// RevenueMetrics returns total revenue and breakdown by category.
type RevenueSummary struct {
	TotalCents int                  `json:"total_cents"`
	TimeSeries []RevenueTimePoint   `json:"time_series"`
	ByCategory []RevenueByCat       `json:"by_category"`
}

type RevenueTimePoint struct {
	Period     string `json:"period"`
	TotalCents int    `json:"total_cents"`
	OrderCount int    `json:"order_count"`
}

type RevenueByCat struct {
	Category   string `json:"category"`
	TotalCents int    `json:"total_cents"`
}

func (r *KPIRepository) RevenueMetrics(from, to time.Time, granularity string) (*RevenueSummary, error) {
	summary := &RevenueSummary{}

	// Total
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(p.amount_cents), 0)
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE p.status = 'paid' AND p.created_at BETWEEN $1 AND $2
	`, from, to).Scan(&summary.TotalCents)
	if err != nil {
		return nil, fmt.Errorf("revenue total: %w", err)
	}

	// Time series
	trunc := granularityToTrunc(granularity)
	tsRows, err := r.db.Query(fmt.Sprintf(`
		SELECT DATE_TRUNC('%s', p.created_at)::date as period,
		       COALESCE(SUM(p.amount_cents), 0),
		       COUNT(DISTINCT p.order_id)
		FROM payments p
		WHERE p.status = 'paid' AND p.created_at BETWEEN $1 AND $2
		GROUP BY period ORDER BY period
	`, trunc), from, to)
	if err != nil {
		return nil, fmt.Errorf("revenue time series: %w", err)
	}
	defer tsRows.Close()

	for tsRows.Next() {
		var rtp RevenueTimePoint
		var period time.Time
		if err := tsRows.Scan(&period, &rtp.TotalCents, &rtp.OrderCount); err != nil {
			return nil, fmt.Errorf("scan revenue time: %w", err)
		}
		rtp.Period = period.Format("2006-01-02")
		summary.TimeSeries = append(summary.TimeSeries, rtp)
	}

	if summary.TimeSeries == nil {
		summary.TimeSeries = []RevenueTimePoint{}
	}

	// By category
	catRows, err := r.db.Query(`
		SELECT COALESCE(pr.category, 'uncategorized') as category,
		       COALESCE(SUM(oi.unit_price_cents * oi.quantity), 0) as total_cents
		FROM order_items oi
		JOIN products pr ON pr.id = oi.product_id
		JOIN orders o ON o.id = oi.order_id
		JOIN payments p ON p.order_id = o.id
		WHERE p.status = 'paid' AND p.created_at BETWEEN $1 AND $2
		GROUP BY pr.category
		ORDER BY total_cents DESC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("revenue by category: %w", err)
	}
	defer catRows.Close()

	for catRows.Next() {
		var rc RevenueByCat
		if err := catRows.Scan(&rc.Category, &rc.TotalCents); err != nil {
			return nil, fmt.Errorf("scan revenue cat: %w", err)
		}
		summary.ByCategory = append(summary.ByCategory, rc)
	}
	if summary.ByCategory == nil {
		summary.ByCategory = []RevenueByCat{}
	}

	return summary, nil
}

// TicketMetrics returns ticket stats.
type TicketMetricsResult struct {
	OpenCount           int     `json:"open_count"`
	InProgressCount     int     `json:"in_progress_count"`
	AvgResolutionHours  float64 `json:"avg_resolution_hours"`
	SLAResponseRate     float64 `json:"sla_response_compliance_pct"`
	SLAResolutionRate   float64 `json:"sla_resolution_compliance_pct"`
	TotalTickets        int     `json:"total_tickets"`
}

func (r *KPIRepository) TicketMetrics() (*TicketMetricsResult, error) {
	result := &TicketMetricsResult{}

	// Counts
	err := r.db.QueryRow(`
		SELECT
		    COALESCE(SUM(CASE WHEN status IN ('open', 'assigned') THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0),
		    COUNT(*)
		FROM tickets
	`).Scan(&result.OpenCount, &result.InProgressCount, &result.TotalTickets)
	if err != nil {
		return nil, fmt.Errorf("ticket counts: %w", err)
	}

	// Avg resolution time (for resolved/closed tickets)
	err = r.db.QueryRow(`
		SELECT COALESCE(
		    AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)) / 3600), 0
		)
		FROM tickets
		WHERE resolved_at IS NOT NULL
	`).Scan(&result.AvgResolutionHours)
	if err != nil {
		return nil, fmt.Errorf("avg resolution: %w", err)
	}

	// SLA compliance rates
	var responseTotal, responseMet int
	err = r.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN sla_response_met = true THEN 1 ELSE 0 END), 0)
		FROM tickets WHERE sla_response_met IS NOT NULL
	`).Scan(&responseTotal, &responseMet)
	if err != nil {
		return nil, fmt.Errorf("sla response compliance: %w", err)
	}
	if responseTotal > 0 {
		result.SLAResponseRate = float64(responseMet) / float64(responseTotal) * 100
	}

	var resolutionTotal, resolutionMet int
	err = r.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN sla_resolution_met = true THEN 1 ELSE 0 END), 0)
		FROM tickets WHERE sla_resolution_met IS NOT NULL
	`).Scan(&resolutionTotal, &resolutionMet)
	if err != nil {
		return nil, fmt.Errorf("sla resolution compliance: %w", err)
	}
	if resolutionTotal > 0 {
		result.SLAResolutionRate = float64(resolutionMet) / float64(resolutionTotal) * 100
	}

	return result, nil
}

// Overview returns a high-level summary for the dashboard.
type OverviewResult struct {
	FillRate      []FillRateResult  `json:"fill_rate"`
	Engagement    *EngagementResult `json:"engagement"`
	Revenue       *RevenueSummary   `json:"revenue"`
	TicketMetrics *TicketMetricsResult `json:"ticket_metrics"`
	TotalMembers  int               `json:"total_members"`
	ActiveMembers int               `json:"active_members"`
}

func (r *KPIRepository) Overview(from, to time.Time, facility string) (*OverviewResult, error) {
	result := &OverviewResult{}
	var err error

	result.FillRate, err = r.FillRateByFacility(from, to, facility)
	if err != nil {
		return nil, err
	}
	if result.FillRate == nil {
		result.FillRate = []FillRateResult{}
	}

	result.Engagement, err = r.EngagementMetrics(from, to)
	if err != nil {
		return nil, err
	}

	result.Revenue, err = r.RevenueMetrics(from, to, "daily")
	if err != nil {
		return nil, err
	}

	result.TicketMetrics, err = r.TicketMetrics()
	if err != nil {
		return nil, err
	}

	// Total and active members
	err = r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'member'`).Scan(&result.TotalMembers)
	if err != nil {
		return nil, fmt.Errorf("total members: %w", err)
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'member' AND status = 'active'`).Scan(&result.ActiveMembers)
	if err != nil {
		return nil, fmt.Errorf("active members: %w", err)
	}

	return result, nil
}

func granularityToTrunc(g string) string {
	switch g {
	case "weekly":
		return "week"
	case "monthly":
		return "month"
	default:
		return "day"
	}
}

func sortMemberTimePoints(pts []MemberTimePoint) {
	for i := 1; i < len(pts); i++ {
		for j := i; j > 0 && pts[j].Period < pts[j-1].Period; j-- {
			pts[j], pts[j-1] = pts[j-1], pts[j]
		}
	}
}
