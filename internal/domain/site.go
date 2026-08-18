package domain

import (
	"strings"
	"time"
)

type SiteStatus string

const (
	SiteActive    SiteStatus = "active"
	SiteSuspended SiteStatus = "suspended"
)

type Site struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Timezone   string     `json:"timezone"`
	Status     SiteStatus `json:"status"`
	DailyLimit int        `json:"daily_limit"`
	CutoffHour int        `json:"cutoff_hour"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Version    int64      `json:"version"`
}

func (s Site) Validate() error {
	if strings.TrimSpace(s.Code) == "" || strings.TrimSpace(s.Name) == "" {
		return FieldError{Field: "site", Message: "code and name are required"}
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return FieldError{Field: "timezone", Message: "is invalid"}
	}
	if s.DailyLimit < 1 || s.DailyLimit > 10000 {
		return FieldError{Field: "daily_limit", Message: "must be between 1 and 10000"}
	}
	if s.CutoffHour < 0 || s.CutoffHour > 23 {
		return FieldError{Field: "cutoff_hour", Message: "must be between 0 and 23"}
	}
	if s.Status != SiteActive && s.Status != SiteSuspended {
		return FieldError{Field: "status", Message: "is invalid"}
	}
	return nil
}

func (s Site) BusinessDay(at time.Time) (string, error) {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return "", err
	}
	local := at.In(loc)
	if local.Hour() < s.CutoffHour {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02"), nil
}

func (s Site) IsOpen() bool { return s.Status == SiteActive }

func (s Site) IsSuspended() bool { return s.Status == SiteSuspended }
