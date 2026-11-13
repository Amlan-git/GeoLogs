package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"geolog/internal/domain"
	"geolog/internal/ids"
	"geolog/internal/service"
)

func registerLocation(router *http.ServeMux, catalog *service.Catalog) {
	router.HandleFunc("POST /vehicles/location", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var body struct {
			VehicleID string   `json:"vehicle_id"`
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
			Timestamp string   `json:"timestamp"`
		}
		if !decodeJSON(w, r, started, &body) {
			return
		}
		id, err := ids.ParseVehicle(body.VehicleID)
		if err != nil {
			writeServiceError(w, started, service.ErrVehicleNotFound)
			return
		}
		if body.Latitude == nil || body.Longitude == nil {
			writeServiceError(w, started, service.ValidationError("latitude and longitude are required"))
			return
		}
		at, err := time.Parse(time.RFC3339, body.Timestamp)
		if err != nil {
			writeServiceError(w, started, service.ValidationError("timestamp must be RFC 3339"))
			return
		}
		result, err := catalog.UpdateLocation(r.Context(), id, domain.Sample{Latitude: *body.Latitude, Longitude: *body.Longitude, Timestamp: at})
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		zones := make([]map[string]any, 0, len(result.Memberships))
		for _, zone := range result.Memberships {
			zones = append(zones, map[string]any{"geofence_id": ids.FormatGeofence(zone.ID), "geofence_name": zone.Name, "status": "inside"})
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"vehicle_id": ids.FormatVehicle(id), "location_updated": result.Updated, "current_geofences": zones})
	})

	router.HandleFunc("GET /vehicles/location/{vehicle_id}", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id, err := ids.ParseVehicle(r.PathValue("vehicle_id"))
		if err != nil {
			writeServiceError(w, started, service.ErrVehicleNotFound)
			return
		}
		result, err := catalog.GetLocation(r.Context(), id)
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		var position map[string]any
		if p := result.Position; p != nil {
			position = map[string]any{"latitude": p.Latitude, "longitude": p.Longitude, "timestamp": p.Timestamp.UTC().Format(time.RFC3339)}
		}
		zones := make([]map[string]any, 0, len(result.Memberships))
		for _, zone := range result.Memberships {
			zones = append(zones, map[string]any{"geofence_id": ids.FormatGeofence(zone.ID), "geofence_name": zone.Name, "category": zone.Category})
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"vehicle_id": ids.FormatVehicle(id), "vehicle_number": result.Number, "current_location": position, "current_geofences": zones})
	})

	router.HandleFunc("GET /violations/history", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			writeServiceError(w, started, service.ValidationError("invalid query parameters"))
			return
		}
		filter, empty, err := parseHistory(query)
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		var crossings []domain.Crossing
		var total int64
		if !empty {
			crossings, total, err = catalog.History(r.Context(), filter)
			if err != nil {
				writeServiceError(w, started, err)
				return
			}
		}
		items := make([]map[string]any, 0, len(crossings))
		for _, item := range crossings {
			items = append(items, map[string]any{
				"id": ids.FormatViolation(item.ID), "vehicle_id": ids.FormatVehicle(item.VehicleID), "vehicle_number": item.VehicleNumber,
				"geofence_id": ids.FormatGeofence(item.GeofenceID), "geofence_name": item.GeofenceName, "event_type": item.EventType,
				"latitude": item.Latitude, "longitude": item.Longitude, "timestamp": item.Timestamp.UTC().Format(time.RFC3339),
			})
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"violations": items, "total_count": total})
	})
}

func parseHistory(query url.Values) (domain.HistoryFilter, bool, error) {
	filter := domain.HistoryFilter{Limit: 50}
	for _, field := range []struct {
		name    string
		minimum int64
		target  *int64
	}{{"limit", 1, &filter.Limit}, {"offset", 0, &filter.Offset}} {
		if query.Has(field.name) {
			value, err := strconv.ParseInt(query.Get(field.name), 10, 64)
			if err != nil || value < field.minimum {
				return filter, false, service.ValidationError("invalid " + field.name)
			}
			*field.target = value
		}
	}
	filter.Limit = min(filter.Limit, 500)
	for _, field := range []struct {
		name   string
		target **time.Time
	}{
		{"start_date", &filter.Start}, {"end_date", &filter.End},
	} {
		if query.Has(field.name) {
			value, err := time.Parse(time.RFC3339, query.Get(field.name))
			if err != nil {
				return filter, false, service.ValidationError(field.name + " must be RFC 3339")
			}
			*field.target = &value
		}
	}
	if filter.Start != nil && filter.End != nil && filter.Start.After(*filter.End) {
		return filter, false, service.ValidationError("start_date must be on or before end_date")
	}
	for _, field := range []struct {
		name   string
		parse  func(string) (int64, error)
		target **int64
	}{
		{"vehicle_id", ids.ParseVehicle, &filter.VehicleID}, {"geofence_id", ids.ParseGeofence, &filter.GeofenceID},
	} {
		if query.Has(field.name) {
			id, err := field.parse(query.Get(field.name))
			if err != nil {
				return filter, true, nil
			}
			*field.target = &id
		}
	}
	return filter, false, nil
}
