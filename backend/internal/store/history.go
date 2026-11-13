package store

import (
	"context"
	"fmt"
	"strings"

	"geolog/internal/domain"
	"github.com/jackc/pgx/v5"
)

func historyConditions(filter domain.HistoryFilter) (string, []any) {
	var conditions []string
	var args []any
	for _, item := range []struct {
		sql     string
		value   any
		enabled bool
	}{
		{"v.vehicle_id =", filter.VehicleID, filter.VehicleID != nil},
		{"v.geofence_id =", filter.GeofenceID, filter.GeofenceID != nil},
		{"v.timestamp >=", filter.Start, filter.Start != nil},
		{"v.timestamp <=", filter.End, filter.End != nil},
	} {
		if item.enabled {
			args = append(args, item.value)
			conditions = append(conditions, fmt.Sprintf("%s $%d", item.sql, len(args)))
		}
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func CountHistory(ctx context.Context, tx pgx.Tx, filter domain.HistoryFilter) (int64, error) {
	where, args := historyConditions(filter)
	var count int64
	err := tx.QueryRow(ctx, "SELECT count(*) FROM violations v"+where, args...).Scan(&count)
	return count, err
}

func ListHistory(ctx context.Context, tx pgx.Tx, filter domain.HistoryFilter) ([]domain.Crossing, error) {
	where, args := historyConditions(filter)
	pagination := fmt.Sprintf(" ORDER BY v.timestamp DESC,v.id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := tx.Query(ctx, `
		SELECT v.id,v.vehicle_id,v.geofence_id,t.vehicle_number,g.name,v.event_type,v.latitude,v.longitude,v.timestamp
		FROM violations v JOIN vehicles t ON t.id=v.vehicle_id JOIN geofences g ON g.id=v.geofence_id`+where+pagination,
		append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Crossing, error) {
		var c domain.Crossing
		err := row.Scan(&c.ID, &c.VehicleID, &c.GeofenceID, &c.VehicleNumber, &c.GeofenceName, &c.EventType, &c.Latitude, &c.Longitude, &c.Timestamp)
		return c, err
	})
}
