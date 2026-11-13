package store

import (
	"context"

	"geolog/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertRule(ctx context.Context, pool *pgxpool.Pool, rule domain.AlertRule) (domain.AlertRule, error) {
	err := pool.QueryRow(ctx, `
		INSERT INTO alert_rules (geofence_id,vehicle_id,event_type)
		VALUES ($1,$2,$3) RETURNING id,status`,
		rule.GeofenceID, rule.VehicleID, rule.EventType).Scan(&rule.ID, &rule.Status)
	return rule, err
}

func ListRules(ctx context.Context, pool *pgxpool.Pool, zone, vehicle *int64) ([]domain.AlertRule, error) {
	rows, err := pool.Query(ctx, `
		SELECT r.id,r.geofence_id,r.vehicle_id,g.name,v.vehicle_number,r.event_type,r.status,r.created_at
		FROM alert_rules r JOIN geofences g ON g.id=r.geofence_id
		LEFT JOIN vehicles v ON v.id=r.vehicle_id
		WHERE ($1::bigint IS NULL OR r.geofence_id=$1) AND ($2::bigint IS NULL OR r.vehicle_id=$2)
		ORDER BY r.id`, zone, vehicle)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[domain.AlertRule])
}

func SetRuleStatus(ctx context.Context, pool *pgxpool.Pool, id int64, status string) (domain.AlertRule, error) {
	var rule domain.AlertRule
	err := pool.QueryRow(ctx, `
		UPDATE alert_rules SET status=$2 WHERE id=$1
		RETURNING id,geofence_id,vehicle_id,event_type,status`, id, status).
		Scan(&rule.ID, &rule.GeofenceID, &rule.VehicleID, &rule.EventType, &rule.Status)
	return rule, err
}

func StoreMatchingAlerts(ctx context.Context, tx pgx.Tx, crossings []int64) ([]domain.Alert, error) {
	rows, err := tx.Query(ctx, `
		WITH matched AS (
			SELECT v.id, array_agg(r.id ORDER BY r.id) AS rule_ids
			FROM violations v JOIN alert_rules r ON r.geofence_id=v.geofence_id
				AND r.status='active'
				AND (r.vehicle_id IS NULL OR r.vehicle_id=v.vehicle_id)
				AND r.event_type IN (v.event_type,'both')
			WHERE v.id=ANY($1::bigint[]) GROUP BY v.id
		), inserted AS (
			INSERT INTO alerts (violation_id,matched_rule_ids)
			SELECT id,rule_ids FROM matched RETURNING id,violation_id
		)
		SELECT a.id,v.id,v.vehicle_id,v.geofence_id,t.vehicle_number,g.name,v.event_type,
		       v.latitude,v.longitude,v.timestamp,t.driver_name,g.category
		FROM inserted a JOIN violations v ON v.id=a.violation_id
		JOIN vehicles t ON t.id=v.vehicle_id JOIN geofences g ON g.id=v.geofence_id
		ORDER BY v.id`, crossings)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Alert, error) {
		var a domain.Alert
		err := row.Scan(&a.ID, &a.Crossing.ID, &a.VehicleID, &a.GeofenceID, &a.VehicleNumber, &a.GeofenceName, &a.EventType,
			&a.Latitude, &a.Longitude, &a.Timestamp, &a.DriverName, &a.Category)
		return a, err
	})
}
