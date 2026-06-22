package gorm

import (
	"context"
	"errors"

	"github.com/showx/ginshow"
	gormdb "gorm.io/gorm"
)

// Check registers a GORM database ping check for readyz.
func Check(name string, db *gormdb.DB) ginshow.NamedCheck {
	return ginshow.NamedCheck{
		Name:   "gorm:" + name,
		Check:  pingCheck(db),
		Detail: poolDetail(db),
	}
}

// Checks builds readiness checks from a map of GORM connections.
func Checks(dbs map[string]*gormdb.DB) []ginshow.NamedCheck {
	checks := make([]ginshow.NamedCheck, 0, len(dbs))
	for name, db := range dbs {
		checks = append(checks, Check(name, db))
	}
	return checks
}

func pingCheck(db *gormdb.DB) ginshow.ReadinessCheck {
	return func(ctx context.Context) error {
		if db == nil {
			return errors.New("gorm db is nil")
		}
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
}

func poolDetail(db *gormdb.DB) func(context.Context) any {
	return func(context.Context) any {
		if db == nil {
			return nil
		}
		sqlDB, err := db.DB()
		if err != nil {
			return map[string]string{"error": err.Error()}
		}
		stats := sqlDB.Stats()
		return map[string]any{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"max_open":         stats.MaxOpenConnections,
			"wait_count":       stats.WaitCount,
			"wait_duration_ms": stats.WaitDuration.Milliseconds(),
		}
	}
}
