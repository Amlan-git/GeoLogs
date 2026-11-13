package service

import (
	"context"
	"errors"
	"time"

	"geolog/internal/domain"
	"geolog/internal/store"
	"github.com/jackc/pgx/v5"
)

var ErrVehicleNotFound = errors.New("vehicle not found")

type LocationResult struct {
	Updated     bool
	Memberships []domain.Membership
	// Crossing IDs become available to the caller only after commit.
	Crossings []domain.Crossing
}

func (c *Catalog) UpdateLocation(ctx context.Context, id int64, point domain.Sample) (result LocationResult, err error) {
	if point.Latitude < -90 || point.Latitude > 90 || point.Longitude < -180 || point.Longitude > 180 {
		return result, ValidationError("latitude must be between -90 and 90 and longitude between -180 and 180")
	}
	if point.Timestamp.After(time.Now().Add(5 * time.Minute)) {
		return result, ValidationError("timestamp must not be more than five minutes in the future")
	}
	// PostgreSQL timestamps have microsecond precision. Compare at the same precision as storage.
	point.Timestamp = point.Timestamp.Truncate(time.Microsecond)
	err = pgx.BeginFunc(ctx, c.pool, func(tx pgx.Tx) error {
		latest, err := store.LockVehicle(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVehicleNotFound
		}
		if err != nil {
			return err
		}
		if err := store.AppendLocation(ctx, tx, id, point); err != nil {
			return err
		}
		if latest != nil && point.Timestamp.Before(*latest) {
			result.Memberships, err = store.ReadMemberships(ctx, tx, id)
			return err
		}
		if err := store.SetCurrentLocation(ctx, tx, id, point); err != nil {
			return err
		}
		current, err := store.FindContainingZones(ctx, tx, point)
		if err != nil {
			return err
		}
		previous, err := store.ReadMemberships(ctx, tx, id)
		if err != nil {
			return err
		}
		entries, exits := domain.DiffMemberships(previous, current)
		if len(entries) > 0 {
			if err := store.AddMemberships(ctx, tx, id, entries); err != nil {
				return err
			}
		}
		if len(exits) > 0 {
			if err := store.RemoveMemberships(ctx, tx, id, exits); err != nil {
				return err
			}
		}
		if len(entries)+len(exits) > 0 {
			result.Crossings, err = store.RecordCrossings(ctx, tx, id, point, entries, exits)
			if err != nil {
				return err
			}
			// Stage 4 matches rules and stores alerts here, using these crossings and tx.
		}
		result.Updated = true
		result.Memberships = current
		return nil
	})
	if err != nil {
		return LocationResult{}, err
	}
	// Stage 4 publishes committed alerts here.
	return result, nil
}

var readSnapshot = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}

func (c *Catalog) GetLocation(ctx context.Context, id int64) (result domain.VehicleLocation, err error) {
	err = pgx.BeginTxFunc(ctx, c.pool, readSnapshot, func(tx pgx.Tx) error {
		result, err = store.ReadPosition(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVehicleNotFound
		}
		if err != nil {
			return err
		}
		result.Memberships, err = store.ReadMemberships(ctx, tx, id)
		return err
	})
	return result, err
}

func (c *Catalog) History(ctx context.Context, filter domain.HistoryFilter) (items []domain.Crossing, total int64, err error) {
	err = pgx.BeginTxFunc(ctx, c.pool, readSnapshot, func(tx pgx.Tx) error {
		total, err = store.CountHistory(ctx, tx, filter)
		if err != nil {
			return err
		}
		items, err = store.ListHistory(ctx, tx, filter)
		return err
	})
	return items, total, err
}
