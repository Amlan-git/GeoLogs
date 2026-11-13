package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Geofence struct {
	ID          int64
	Name        string
	Description string
	Category    string
	Coordinates json.RawMessage
	CreatedAt   time.Time
}

type Vehicle struct {
	ID        int64
	Number    string
	Driver    string
	Type      string
	Phone     string
	CreatedAt time.Time
}

func ValidateGeofence(zone Geofence) ([][2]float64, error) {
	if strings.TrimSpace(zone.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.ContainsRune(zone.Name, 0) || strings.ContainsRune(zone.Description, 0) {
		return nil, fmt.Errorf("name and description cannot contain a null character")
	}
	switch zone.Category {
	case "delivery_zone", "restricted_zone", "toll_zone", "customer_area":
	default:
		return nil, fmt.Errorf("category must be delivery_zone, restricted_zone, toll_zone, or customer_area")
	}
	var pairs [][]*float64
	if err := json.Unmarshal(zone.Coordinates, &pairs); err != nil || pairs == nil {
		return nil, fmt.Errorf("coordinates must be an array of latitude and longitude pairs")
	}
	for _, pair := range pairs {
		if len(pair) != 2 || pair[0] == nil || pair[1] == nil {
			return nil, fmt.Errorf("each coordinate must contain exactly two numbers")
		}
	}
	ring := make([][2]float64, len(pairs))
	for i, pair := range pairs {
		lat, lon := *pair[0], *pair[1]
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, fmt.Errorf("latitude must be between -90 and 90 and longitude between -180 and 180")
		}
		ring[i] = [2]float64{lat, lon}
	}
	if len(ring) < 4 {
		return nil, fmt.Errorf("coordinates must contain at least four points including the closing point")
	}
	if ring[0] != ring[len(ring)-1] {
		return nil, fmt.Errorf("the first and last coordinates must be identical")
	}
	distinct := make(map[[2]float64]struct{})
	for _, point := range ring {
		distinct[point] = struct{}{}
	}
	if len(distinct) < 3 {
		return nil, fmt.Errorf("the polygon must contain at least three distinct vertices")
	}
	return ring, nil
}

func ValidateVehicle(vehicle Vehicle) error {
	for _, field := range []struct{ name, value string }{
		{"vehicle_number", vehicle.Number},
		{"driver_name", vehicle.Driver},
		{"vehicle_type", vehicle.Type},
		{"phone", vehicle.Phone},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
		if strings.ContainsRune(field.value, 0) {
			return fmt.Errorf("%s cannot contain a null character", field.name)
		}
	}
	return nil
}
