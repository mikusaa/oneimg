package services

import (
	"oneimg/backend/config"
	"oneimg/backend/database"
)

type Services struct {
	Config   *config.Config
	Accounts *AccountService
	Passkeys *PasskeyService
	Tokens   *TokenService
	Tags     *TagService
	Images   *ImageService
	Users    *UserService
	Settings *SettingsService
	Stats    *StatsService
	Storage  *StorageService
	Uploads  *UploadService
}

func New(db *database.Database, cfg *config.Config) *Services {
	result := &Services{
		Config: cfg,
		Tokens: NewTokenService(db.DB, cfg.ConfigSecret),
	}
	result.Accounts = NewAccountService(db.DB)
	result.Passkeys = NewPasskeyService(db.DB, result.Accounts)
	result.Tags = NewTagService(db.DB)
	result.Images = NewImageService(db.DB)
	result.Users = NewUserService(db.DB)
	result.Settings = NewSettingsService(db.DB)
	result.Stats = NewStatsService(db.DB)
	result.Storage = NewStorageService(db.DB)
	result.Uploads = NewUploadService(db.DB)
	return result
}
