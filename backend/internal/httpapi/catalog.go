package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"geolog/internal/domain"
	"geolog/internal/ids"
	"geolog/internal/service"
)

func registerCatalog(router *http.ServeMux, catalog *service.Catalog) {
	router.HandleFunc("POST /geofences", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var body struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Category    string          `json:"category"`
			Coordinates json.RawMessage `json:"coordinates"`
		}
		if !decodeJSON(w, r, started, &body) {
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		id, err := catalog.CreateGeofence(r.Context(), domain.Geofence{
			Name: body.Name, Description: body.Description, Category: body.Category, Coordinates: body.Coordinates,
		})
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		writeJSON(w, started, http.StatusCreated, map[string]any{
			"id": ids.FormatGeofence(id), "name": body.Name, "status": "active",
		})
	})

	router.HandleFunc("GET /geofences", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		zones, err := catalog.ListGeofences(r.Context(), r.URL.Query().Get("category"))
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		items := make([]map[string]any, 0, len(zones))
		for _, zone := range zones {
			items = append(items, map[string]any{
				"id": ids.FormatGeofence(zone.ID), "name": zone.Name, "description": zone.Description,
				"coordinates": zone.Coordinates, "category": zone.Category,
				"created_at": zone.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"geofences": items})
	})

	router.HandleFunc("POST /vehicles", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var body struct {
			Number string `json:"vehicle_number"`
			Driver string `json:"driver_name"`
			Type   string `json:"vehicle_type"`
			Phone  string `json:"phone"`
		}
		if !decodeJSON(w, r, started, &body) {
			return
		}
		body.Number = strings.TrimSpace(body.Number)
		body.Driver = strings.TrimSpace(body.Driver)
		body.Type = strings.TrimSpace(body.Type)
		body.Phone = strings.TrimSpace(body.Phone)
		id, err := catalog.CreateVehicle(r.Context(), domain.Vehicle{
			Number: body.Number, Driver: body.Driver, Type: body.Type, Phone: body.Phone,
		})
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		writeJSON(w, started, http.StatusCreated, map[string]any{
			"id": ids.FormatVehicle(id), "vehicle_number": body.Number, "status": "active",
		})
	})

	router.HandleFunc("GET /vehicles", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		vehicles, err := catalog.ListVehicles(r.Context())
		if err != nil {
			writeServiceError(w, started, err)
			return
		}
		items := make([]map[string]any, 0, len(vehicles))
		for _, vehicle := range vehicles {
			items = append(items, map[string]any{
				"id": ids.FormatVehicle(vehicle.ID), "vehicle_number": vehicle.Number, "driver_name": vehicle.Driver,
				"vehicle_type": vehicle.Type, "phone": vehicle.Phone, "status": "active",
				"created_at": vehicle.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		writeJSON(w, started, http.StatusOK, map[string]any{"vehicles": items})
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, started time.Time, target any) bool {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err == nil {
		if err := decoder.Decode(new(json.RawMessage)); err == io.EOF {
			return true
		}
	}
	writeJSON(w, started, http.StatusBadRequest, map[string]any{"error": "request body must contain one valid JSON object of at most 1 MB"})
	return false
}

func writeServiceError(w http.ResponseWriter, started time.Time, err error) {
	status, message := http.StatusInternalServerError, "Unexpected server error"
	var validation service.ValidationError
	switch {
	case errors.As(err, &validation):
		status, message = http.StatusBadRequest, validation.Error()
	case errors.Is(err, service.ErrVehicleNotFound):
		status, message = http.StatusNotFound, err.Error()
	case errors.Is(err, service.ErrConflict):
		status, message = http.StatusConflict, err.Error()
	default:
		log.Printf("Catalog request failed: %v", err)
	}
	writeJSON(w, started, status, map[string]any{"error": message})
}
