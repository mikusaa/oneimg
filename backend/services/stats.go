package services

import (
	"time"

	"oneimg/backend/models"

	"gorm.io/gorm"
)

type StatsService struct{ db *gorm.DB }

type TrendPoint struct {
	Date  string
	Count int64
}

type FormatStat struct {
	Format string
	Count  int64
	Size   int64
}

type SizeStat struct {
	Range string
	Count int64
}

type DashboardStats struct {
	TotalImages      int64
	TotalSize        int64
	TodayUploads     int64
	MonthUploads     int64
	RecentImages     []ImageRecord
	UploadTrend      []TrendPoint
	FormatStats      []FormatStat
	SizeDistribution []SizeStat
}

func NewStatsService(db *gorm.DB) *StatsService { return &StatsService{db: db} }

func (s *StatsService) imageScope(user models.User) *gorm.DB {
	query := s.db.Model(&models.Image{})
	if user.Role != models.RoleAdmin {
		query = query.Where("user_id = ?", user.ID)
	}
	return query
}

func (s *StatsService) Dashboard(user models.User) (DashboardStats, error) {
	scope := s.imageScope(user)
	var result DashboardStats
	if err := scope.Count(&result.TotalImages).Error; err != nil {
		return result, err
	}
	if err := scope.Select("COALESCE(SUM(file_size), 0)").Scan(&result.TotalSize).Error; err != nil {
		return result, err
	}
	now := time.Now()
	today := localDay(now)
	result.TodayUploads = countBetween(scope, today, today.AddDate(0, 0, 1))
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	result.MonthUploads = countBetween(scope, month, month.AddDate(0, 1, 0))
	var recent []models.Image
	if err := scope.Order("created_at DESC").Limit(10).Find(&recent).Error; err != nil {
		return result, err
	}
	var err error
	result.RecentImages, err = s.Images().attachMetadata(recent)
	if err != nil {
		return result, err
	}
	result.UploadTrend = trend(scope, "day", 7)
	if err := scope.Select("mime_type format, COUNT(*) count, COALESCE(SUM(file_size), 0) size").Group("mime_type").Scan(&result.FormatStats).Error; err != nil {
		return result, err
	}
	ranges := []struct {
		name     string
		min, max int64
	}{
		{"< 100KB", 0, 100 * 1024}, {"100KB - 500KB", 100 * 1024, 500 * 1024},
		{"500KB - 1MB", 500 * 1024, 1024 * 1024}, {"1MB - 5MB", 1024 * 1024, 5 * 1024 * 1024},
		{"5MB - 10MB", 5 * 1024 * 1024, 10 * 1024 * 1024}, {">= 10MB", 10 * 1024 * 1024, 0},
	}
	for _, item := range ranges {
		var count int64
		query := s.imageScope(user).Where("file_size >= ?", item.min)
		if item.max > 0 {
			query = query.Where("file_size < ?", item.max)
		}
		if err := query.Count(&count).Error; err != nil {
			return result, err
		}
		result.SizeDistribution = append(result.SizeDistribution, SizeStat{Range: item.name, Count: count})
	}
	return result, nil
}

// Images returns an image service sharing the same database transaction scope.
func (s *StatsService) Images() *ImageService { return NewImageService(s.db) }

func (s *StatsService) ImageTrend(user models.User, period string) []TrendPoint {
	counts := map[string]int{"day": 30, "week": 12, "month": 12, "year": 5}
	return trend(s.imageScope(user), period, counts[period])
}

func trend(scope *gorm.DB, period string, count int) []TrendPoint {
	now := time.Now()
	result := make([]TrendPoint, 0, count)
	for i := count - 1; i >= 0; i-- {
		var start, end time.Time
		var label string
		switch period {
		case "week":
			day := localDay(now)
			monday := day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
			start = monday.AddDate(0, 0, -i*7)
			end = start.AddDate(0, 0, 7)
			label = start.Format("2006-01-02")
		case "month":
			value := now.AddDate(0, -i, 0)
			start = time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
			end = start.AddDate(0, 1, 0)
			label = start.Format("2006-01")
		case "year":
			value := now.AddDate(-i, 0, 0)
			start = time.Date(value.Year(), time.January, 1, 0, 0, 0, 0, value.Location())
			end = start.AddDate(1, 0, 0)
			label = start.Format("2006")
		default:
			start = localDay(now.AddDate(0, 0, -i))
			end = start.AddDate(0, 0, 1)
			label = start.Format("2006-01-02")
		}
		result = append(result, TrendPoint{Date: label, Count: countBetween(scope, start, end)})
	}
	return result
}

func localDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func countBetween(scope *gorm.DB, start, end time.Time) int64 {
	var count int64
	scope.Where("julianday(created_at) >= julianday(?) AND julianday(created_at) < julianday(?)", start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)).Count(&count)
	return count
}
