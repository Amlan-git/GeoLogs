package service

import (
	"context"
	"errors"

	"geolog/internal/domain"
	"geolog/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrGeofenceNotFound = errors.New("geofence not found")
	ErrRuleNotFound     = errors.New("alert rule not found")
)

type AlertPublisher interface {
	Publish(domain.Alert)
}

func (c *Catalog) CreateRule(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	switch rule.EventType {
	case "entry", "exit", "both":
	default:
		return rule, ValidationError("event_type must be entry, exit, or both")
	}
	result, err := store.InsertRule(ctx, c.pool, rule)
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.Code == "23503" {
		switch dbErr.ConstraintName {
		case "alert_rules_geofence_id_fkey":
			return result, ErrGeofenceNotFound
		case "alert_rules_vehicle_id_fkey":
			return result, ErrVehicleNotFound
		}
	}
	return result, err
}

func (c *Catalog) ListRules(ctx context.Context, zone, vehicle *int64) ([]domain.AlertRule, error) {
	return store.ListRules(ctx, c.pool, zone, vehicle)
}

func (c *Catalog) ToggleRule(ctx context.Context, id int64, status string) (domain.AlertRule, error) {
	if status != "active" && status != "inactive" {
		return domain.AlertRule{}, ValidationError("status must be active or inactive")
	}
	rule, err := store.SetRuleStatus(ctx, c.pool, id, status)
	if errors.Is(err, pgx.ErrNoRows) {
		return rule, ErrRuleNotFound
	}
	return rule, err
}
