package v1

import (
	"net/http"

	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
)

type trendDTO struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
type formatStatsDTO struct {
	Format string `json:"format"`
	Count  int64  `json:"count"`
	Size   int64  `json:"size"`
}
type sizeStatsDTO struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

func (s *Server) dashboardStats(c *gin.Context) {
	user, _ := currentUser(c)
	item, err := s.services.Stats.Dashboard(*user)
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "stats_read_failed", "读取统计数据失败")
		return
	}
	recent := make([]imageDTO, 0, len(item.RecentImages))
	for _, image := range item.RecentImages {
		recent = append(recent, toImageDTO(image))
	}
	trends := toTrendDTOs(item.UploadTrend)
	formats := make([]formatStatsDTO, 0, len(item.FormatStats))
	for _, value := range item.FormatStats {
		formats = append(formats, formatStatsDTO{Format: value.Format, Count: value.Count, Size: value.Size})
	}
	sizes := make([]sizeStatsDTO, 0, len(item.SizeDistribution))
	for _, value := range item.SizeDistribution {
		sizes = append(sizes, sizeStatsDTO{Range: value.Range, Count: value.Count})
	}
	writeData(c, http.StatusOK, gin.H{"total_images": item.TotalImages, "total_size": item.TotalSize, "today_uploads": item.TodayUploads, "month_uploads": item.MonthUploads, "recent_images": recent, "upload_trend": trends, "format_stats": formats, "size_distribution": sizes}, nil)
}

func (s *Server) imageStats(c *gin.Context) {
	period := c.DefaultQuery("period", "month")
	if period != "day" && period != "week" && period != "month" && period != "year" {
		writeProblem(c, http.StatusUnprocessableEntity, "invalid_query_parameter", "period 只能是 day、week、month 或 year")
		return
	}
	user, _ := currentUser(c)
	writeData(c, http.StatusOK, toTrendDTOs(s.services.Stats.ImageTrend(*user, period)), nil)
}

func toTrendDTOs(items []services.TrendPoint) []trendDTO {
	result := make([]trendDTO, 0, len(items))
	for _, item := range items {
		result = append(result, trendDTO{Date: item.Date, Count: item.Count})
	}
	return result
}
