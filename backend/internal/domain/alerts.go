package domain

import "time"

type AlertRule struct {
	ID, GeofenceID    int64
	VehicleID         *int64
	GeofenceName      string
	VehicleNumber     *string
	EventType, Status string
	CreatedAt         time.Time
}

type Alert struct {
	ID int64
	Crossing
	DriverName, Category string
}
