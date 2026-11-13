package domain

import "time"

type Sample struct {
	Latitude, Longitude float64
	Timestamp           time.Time
}

type Membership struct {
	ID             int64
	Name, Category string
}

type VehicleLocation struct {
	Number      string
	Position    *Sample
	Memberships []Membership
}

type Crossing struct {
	ID, VehicleID, GeofenceID              int64
	VehicleNumber, GeofenceName, EventType string
	Sample
}

type HistoryFilter struct {
	VehicleID, GeofenceID *int64
	Start, End            *time.Time
	Limit, Offset         int64
}

func DiffMemberships(previous, current []Membership) (entries, exits []int64) {
	oldIDs := make(map[int64]bool, len(previous))
	newIDs := make(map[int64]bool, len(current))
	for _, zone := range previous {
		oldIDs[zone.ID] = true
	}
	for _, zone := range current {
		newIDs[zone.ID] = true
		if !oldIDs[zone.ID] {
			entries = append(entries, zone.ID)
		}
	}
	for _, zone := range previous {
		if !newIDs[zone.ID] {
			exits = append(exits, zone.ID)
		}
	}
	return entries, exits
}
