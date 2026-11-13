package store

import (
	"context"
	"time"

	"geolog/internal/domain"
	"github.com/jackc/pgx/v5"
)

func LockVehicle(ctx context.Context, tx pgx.Tx, id int64) (*time.Time, error) {
	var latest *time.Time
	err := tx.QueryRow(ctx, "SELECT last_location_at FROM vehicles WHERE id = $1 FOR UPDATE", id).Scan(&latest)
	return latest, err
}

func AppendLocation(ctx context.Context, tx pgx.Tx, id int64, point domain.Sample) error {
	_, err := tx.Exec(ctx, `INSERT INTO vehicle_locations (vehicle_id,latitude,longitude,recorded_at) VALUES ($1,$2,$3,$4)`, id, point.Latitude, point.Longitude, point.Timestamp)
	return err
}

func SetCurrentLocation(ctx context.Context, tx pgx.Tx, id int64, point domain.Sample) error {
	_, err := tx.Exec(ctx, `UPDATE vehicles SET last_latitude=$2,last_longitude=$3,last_location_at=$4 WHERE id=$1`, id, point.Latitude, point.Longitude, point.Timestamp)
	return err
}

func ReadPosition(ctx context.Context, tx pgx.Tx, id int64) (domain.VehicleLocation, error) {
	var result domain.VehicleLocation
	var lat, lon *float64
	var at *time.Time
	err := tx.QueryRow(ctx, `SELECT vehicle_number,last_latitude,last_longitude,last_location_at FROM vehicles WHERE id=$1`, id).Scan(&result.Number, &lat, &lon, &at)
	if err == nil && at != nil {
		result.Position = &domain.Sample{Latitude: *lat, Longitude: *lon, Timestamp: *at}
	}
	return result, err
}

func ReadMemberships(ctx context.Context, tx pgx.Tx, id int64) ([]domain.Membership, error) {
	rows, err := tx.Query(ctx, `
		SELECT g.id,g.name,g.category FROM vehicle_geofence_memberships m
		JOIN geofences g ON g.id=m.geofence_id WHERE m.vehicle_id=$1 ORDER BY g.id`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Membership])
}

func FindContainingZones(ctx context.Context, tx pgx.Tx, point domain.Sample) ([]domain.Membership, error) {
	rows, err := tx.Query(ctx, `
		SELECT id,name,category FROM geofences
		WHERE ST_Covers(geometry,ST_SetSRID(ST_MakePoint($1,$2),4326)) ORDER BY id`, point.Longitude, point.Latitude)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Membership])
}

func AddMemberships(ctx context.Context, tx pgx.Tx, id int64, entries []int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO vehicle_geofence_memberships (vehicle_id,geofence_id) SELECT $1,unnest($2::bigint[])`, id, entries)
	return err
}

func RemoveMemberships(ctx context.Context, tx pgx.Tx, id int64, exits []int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM vehicle_geofence_memberships WHERE vehicle_id=$1 AND geofence_id=ANY($2::bigint[])`, id, exits)
	return err
}

func RecordCrossings(ctx context.Context, tx pgx.Tx, id int64, point domain.Sample, entries, exits []int64) ([]domain.Crossing, error) {
	rows, err := tx.Query(ctx, `
		INSERT INTO violations (vehicle_id,geofence_id,event_type,latitude,longitude,timestamp)
		SELECT $1, geofence_id,event_type,$2,$3,$4 FROM (
			SELECT unnest($5::bigint[]) AS geofence_id, 'entry' AS event_type
			UNION ALL
			SELECT unnest($6::bigint[]), 'exit'
		) AS changes RETURNING id,geofence_id,event_type`, id, point.Latitude, point.Longitude, point.Timestamp, entries, exits)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Crossing, error) {
		crossing := domain.Crossing{VehicleID: id, Sample: point}
		err := row.Scan(&crossing.ID, &crossing.GeofenceID, &crossing.EventType)
		return crossing, err
	})
}
