package store

import (
	"context"

	"geolog/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertGeofence(ctx context.Context, pool *pgxpool.Pool, zone domain.Geofence, polygon string) (id int64, invalidReason string, err error) {
	// Validate and insert the same geometry in one statement.
	err = pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT ST_GeomFromText($5, 4326) AS geometry
		), checked AS MATERIALIZED (
			SELECT geometry, (ST_IsValidDetail(geometry)).* FROM candidate
		), inserted AS (
			INSERT INTO geofences (name, description, category, coordinates, geometry)
			SELECT $1, $2, $3, $4, geometry FROM checked WHERE valid
			RETURNING id
		)
		SELECT COALESCE((SELECT id FROM inserted), 0), COALESCE(reason, '') FROM checked
	`, zone.Name, zone.Description, zone.Category, zone.Coordinates, polygon).Scan(&id, &invalidReason)
	return
}

func ListGeofences(ctx context.Context, pool *pgxpool.Pool, category string) ([]domain.Geofence, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, name, description, category, coordinates, created_at
		FROM geofences WHERE ($1 = '' OR category = $1) ORDER BY id
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	zones := make([]domain.Geofence, 0)
	for rows.Next() {
		var zone domain.Geofence
		if err := rows.Scan(&zone.ID, &zone.Name, &zone.Description, &zone.Category, &zone.Coordinates, &zone.CreatedAt); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}

func InsertVehicle(ctx context.Context, pool *pgxpool.Pool, vehicle domain.Vehicle) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO vehicles (vehicle_number, driver_name, vehicle_type, phone)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, vehicle.Number, vehicle.Driver, vehicle.Type, vehicle.Phone).Scan(&id)
	return id, err
}

func ListVehicles(ctx context.Context, pool *pgxpool.Pool) ([]domain.Vehicle, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, created_at
		FROM vehicles ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vehicles := make([]domain.Vehicle, 0)
	for rows.Next() {
		var vehicle domain.Vehicle
		if err := rows.Scan(&vehicle.ID, &vehicle.Number, &vehicle.Driver, &vehicle.Type, &vehicle.Phone, &vehicle.CreatedAt); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, vehicle)
	}
	return vehicles, rows.Err()
}
