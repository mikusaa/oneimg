package services

import (
	"errors"
	"strings"
	"unicode/utf8"

	"oneimg/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTagNameInvalid = errors.New("tag name must contain between 1 and 10 characters")
	ErrTagConflict    = errors.New("tag name already exists")
)

type TagService struct{ db *gorm.DB }

func NewTagService(db *gorm.DB) *TagService { return &TagService{db: db} }

func validateTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 10 {
		return "", ErrTagNameInvalid
	}
	return name, nil
}

func (s *TagService) List() ([]models.Tags, error) {
	items := make([]models.Tags, 0)
	err := s.db.Order("id ASC").Find(&items).Error
	return items, err
}

func (s *TagService) Create(name string) (models.Tags, error) {
	name, err := validateTagName(name)
	if err != nil {
		return models.Tags{}, err
	}
	item := models.Tags{Name: name}
	if err := s.db.Create(&item).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.Tags{}, ErrTagConflict
		}
		return models.Tags{}, err
	}
	return item, nil
}

func (s *TagService) Update(id int, name string) (models.Tags, error) {
	name, err := validateTagName(name)
	if err != nil {
		return models.Tags{}, err
	}
	var item models.Tags
	if err := s.db.First(&item, id).Error; err != nil {
		return models.Tags{}, err
	}
	if err := s.db.Model(&item).Update("name", name).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.Tags{}, ErrTagConflict
		}
		return models.Tags{}, err
	}
	item.Name = name
	return item, nil
}

func (s *TagService) Delete(id int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item models.Tags
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if err := tx.Where("tag_id = ?", id).Delete(&models.ImageToTags{}).Error; err != nil {
			return err
		}
		return tx.Delete(&item).Error
	})
}

func (s *TagService) UpdateImageTags(imageIDs, addTagIDs, removeTagIDs []int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var imageCount int64
		if err := tx.Model(&models.Image{}).Where("id IN ?", imageIDs).Count(&imageCount).Error; err != nil {
			return err
		}
		if imageCount != int64(len(imageIDs)) {
			return gorm.ErrRecordNotFound
		}
		allTags := append(append([]int{}, addTagIDs...), removeTagIDs...)
		if len(allTags) > 0 {
			var tagCount int64
			if err := tx.Model(&models.Tags{}).Where("id IN ?", allTags).Count(&tagCount).Error; err != nil {
				return err
			}
			unique := uniqueInts(allTags)
			if tagCount != int64(len(unique)) {
				return gorm.ErrRecordNotFound
			}
		}
		if len(removeTagIDs) > 0 {
			if err := tx.Where("image_id IN ? AND tag_id IN ?", imageIDs, removeTagIDs).Delete(&models.ImageToTags{}).Error; err != nil {
				return err
			}
		}
		links := make([]models.ImageToTags, 0, len(imageIDs)*len(addTagIDs))
		for _, imageID := range imageIDs {
			for _, tagID := range addTagIDs {
				links = append(links, models.ImageToTags{ImageId: imageID, TagId: tagID})
			}
		}
		if len(links) > 0 {
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error
		}
		return nil
	})
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
