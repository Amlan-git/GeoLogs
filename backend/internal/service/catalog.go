package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"geolog/internal/domain"
	"geolog/internal/store"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("vehicle number is already registered")

type ValidationError string

func (e ValidationError) Error() string { return string(e) }

type Catalog struct {
	pool      *pgxpool.Pool
	publisher AlertPublisher
}

func NewCatalog(pool *pgxpool.Pool, publisher AlertPublisher) *Catalog {
	return &Catalog{pool: pool, publisher: publisher}
}

func (c *Catalog) CreateGeofence(ctx context.Context, zone domain.Geofence) (int64, error) {
	ring, err := domain.ValidateGeofence(zone)
	if err != nil {
		return 0, ValidationError(err.Error())
	}
	var polygon strings.Builder
	polygon.WriteString("POLYGON((")
	for i, point := range ring {
		if i > 0 {
			polygon.WriteByte(',')
		}
		polygon.WriteString(strconv.FormatFloat(point[1], 'f', -1, 64))
		polygon.WriteByte(' ')
		polygon.WriteString(strconv.FormatFloat(point[0], 'f', -1, 64))
	}
	polygon.WriteString("))")
	id, reason, err := store.InsertGeofence(ctx, c.pool, zone, polygon.String())
	if err != nil {
		return 0, err
	}
	if id == 0 {
		log.Printf("Invalid geofence geometry: %q", reason)
		return 0, ValidationError("polygon must enclose an area without crossing itself")
	}
	return id, nil
}

func (c *Catalog) ListGeofences(ctx context.Context, category string) ([]domain.Geofence, error) {
	return store.ListGeofences(ctx, c.pool, category)
}

func (c *Catalog) CreateVehicle(ctx context.Context, vehicle domain.Vehicle) (int64, error) {
	if err := domain.ValidateVehicle(vehicle); err != nil {
		return 0, ValidationError(err.Error())
	}
	id, err := store.InsertVehicle(ctx, c.pool, vehicle)
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.Code == "23505" && dbErr.ConstraintName == "vehicles_number_unique_idx" {
		return 0, ErrConflict
	}
	return id, err
}

func (c *Catalog) ListVehicles(ctx context.Context) ([]domain.Vehicle, error) {
	return store.ListVehicles(ctx, c.pool)
}
