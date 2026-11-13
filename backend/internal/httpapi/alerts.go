package httpapi

import (
	"net/http"
	"net/url"
	"time"

	"geolog/internal/domain"
	"geolog/internal/ids"
	"geolog/internal/service"
)

func ruleResponse(rule domain.AlertRule) map[string]any {
	var vehicle any
	if rule.VehicleID != nil {
		vehicle = ids.FormatVehicle(*rule.VehicleID)
	}
	return map[string]any{
		"alert_id": ids.FormatAlertRule(rule.ID), "geofence_id": ids.FormatGeofence(rule.GeofenceID),
		"vehicle_id": vehicle, "event_type": rule.EventType, "status": rule.Status,
	}
}

func registerAlerts(router *http.ServeMux, catalog *service.Catalog) {
	router.HandleFunc("POST /alerts/configure", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var body struct {
			GeofenceID string  `json:"geofence_id"`
			VehicleID  *string `json:"vehicle_id"`
			EventType  string  `json:"event_type"`
		}
		if !decodeJSON(w, r, started, &body) {
			return
		}
		zone, err := ids.ParseGeofence(body.GeofenceID)
		if err != nil {
			writeServiceError(w, started, service.ErrGeofenceNotFound)
			return
		}
		rule := domain.AlertRule{GeofenceID: zone, EventType: body.EventType}
		if body.VehicleID != nil {
			vehicle, err := ids.ParseVehicle(*body.VehicleID)
			if err != nil {
				writeServiceError(w, started, service.ErrVehicleNotFound)
				return
			}
			rule.VehicleID = &vehicle
		}
		created, err := catalog.CreateRule(r.Context(), rule)
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		writeJSON(w, started, http.StatusCreated, ruleResponse(created))
	})
	router.HandleFunc("GET /alerts", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			writeServiceError(w, started, service.ValidationError("invalid query parameters"))
			return
		}
		items := make([]map[string]any, 0)
		var zone, vehicle *int64
		for _, field := range []struct {
			name   string
			parse  func(string) (int64, error)
			target **int64
		}{
			{"geofence_id", ids.ParseGeofence, &zone}, {"vehicle_id", ids.ParseVehicle, &vehicle},
		} {
			if query.Has(field.name) {
				id, err := field.parse(query.Get(field.name))
				if err != nil {
					writeJSON(w, started, http.StatusOK, map[string]any{"alerts": items})
					return
				}
				*field.target = &id
			}
		}
		rules, err := catalog.ListRules(r.Context(), zone, vehicle)
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		for _, rule := range rules {
			item := ruleResponse(rule)
			item["geofence_name"] = rule.GeofenceName
			item["vehicle_number"] = rule.VehicleNumber
			item["created_at"] = rule.CreatedAt.UTC().Format(time.RFC3339)
			items = append(items, item)
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"alerts": items})
	})
	router.HandleFunc("PATCH /alerts/{alert_id}", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id, err := ids.ParseAlertRule(r.PathValue("alert_id"))
		if err != nil {
			writeServiceError(w, started, service.ErrRuleNotFound)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if !decodeJSON(w, r, started, &body) {
			return
		}
		rule, err := catalog.ToggleRule(r.Context(), id, body.Status)
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		writeJSON(w, started, http.StatusOK, ruleResponse(rule))
	})
}
