package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestValidateGeofence(t *testing.T) {
	const sample = `[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]`
	for _, tc := range []struct {
		name, zoneName, description, category, coordinates, want string
	}{
		{"brief sample", "Downtown Delivery Zone", "Main delivery area for downtown customers", "delivery_zone", sample, ""},
		{"blank name", " \t", "", "delivery_zone", sample, "name is required"},
		{"null character in name", "Zone\x00", "", "delivery_zone", sample, "null character"},
		{"null character in description", "Zone", "text\x00", "delivery_zone", sample, "null character"},
		{"missing category", "Zone", "", "", sample, "category must be"},
		{"invalid category", "Zone", "", "other", sample, "category must be"},
		{"missing coordinates", "Zone", "", "delivery_zone", "", "must be an array"},
		{"null coordinates", "Zone", "", "delivery_zone", "null", "must be an array"},
		{"malformed coordinates", "Zone", "", "delivery_zone", "[", "must be an array"},
		{"wrong coordinate type", "Zone", "", "delivery_zone", `[["0",0]]`, "must be an array"},
		{"null pair", "Zone", "", "delivery_zone", "[null]", "exactly two numbers"},
		{"null latitude", "Zone", "", "delivery_zone", "[[null,0]]", "exactly two numbers"},
		{"null longitude", "Zone", "", "delivery_zone", "[[0,null]]", "exactly two numbers"},
		{"short pair", "Zone", "", "delivery_zone", "[[0]]", "exactly two numbers"},
		{"long pair", "Zone", "", "delivery_zone", "[[0,0,0]]", "exactly two numbers"},
		{"latitude too high", "Zone", "", "delivery_zone", "[[91,0]]", "latitude must be"},
		{"latitude too low", "Zone", "", "delivery_zone", "[[-91,0]]", "latitude must be"},
		{"longitude too high", "Zone", "", "delivery_zone", "[[0,181]]", "longitude between"},
		{"longitude too low", "Zone", "", "delivery_zone", "[[0,-181]]", "longitude between"},
		{"too few points", "Zone", "", "delivery_zone", "[[0,0],[1,0],[0,0]]", "at least four points"},
		{"unclosed ring", "Zone", "", "delivery_zone", "[[0,0],[1,0],[1,1],[0,1]]", "must be identical"},
		{"too few distinct vertices", "Zone", "", "delivery_zone", "[[0,0],[1,0],[1,0],[0,0]]", "distinct vertices"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateGeofence(Geofence{
				Name: tc.zoneName, Description: tc.description, Category: tc.category,
				Coordinates: json.RawMessage(tc.coordinates),
			})
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || got != nil {
					t.Fatalf("got %v, %v; want rejection containing %q", got, err, tc.want)
				}
				return
			}
			want := [][2]float64{
				{37.7749, -122.4194}, {37.7849, -122.4194}, {37.7849, -122.4094},
				{37.7749, -122.4094}, {37.7749, -122.4194},
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestValidateVehicle(t *testing.T) {
	sample := Vehicle{Number: "KA-01-AB-1234", Driver: "John Doe", Type: "truck", Phone: "+1234567890"}
	t.Run("brief sample", func(t *testing.T) {
		if err := ValidateVehicle(sample); err != nil {
			t.Fatal(err)
		}
	})
	for _, field := range []struct {
		name string
		set  func(*Vehicle, string)
	}{
		{"vehicle_number", func(v *Vehicle, s string) { v.Number = s }},
		{"driver_name", func(v *Vehicle, s string) { v.Driver = s }},
		{"vehicle_type", func(v *Vehicle, s string) { v.Type = s }},
		{"phone", func(v *Vehicle, s string) { v.Phone = s }},
	} {
		for _, invalid := range []struct{ name, value, suffix string }{
			{"missing", "", " is required"},
			{"blank", " \t", " is required"},
			{"null character", "valid\x00", " cannot contain a null character"},
		} {
			t.Run(field.name+"/"+invalid.name, func(t *testing.T) {
				vehicle := sample
				field.set(&vehicle, invalid.value)
				want := field.name + invalid.suffix
				if err := ValidateVehicle(vehicle); err == nil || err.Error() != want {
					t.Fatalf("got %v; want %q", err, want)
				}
			})
		}
	}
}
