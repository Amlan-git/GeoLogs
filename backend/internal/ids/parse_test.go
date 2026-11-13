package ids

import "testing"

func TestPublicIDs(t *testing.T) {
	for _, tc := range []struct {
		prefix string
		format func(int64) string
		parse  func(string) (int64, error)
	}{
		{"geo_", FormatGeofence, ParseGeofence},
		{"veh_", FormatVehicle, ParseVehicle},
		{"alert_", FormatAlertRule, ParseAlertRule},
		{"viol_", FormatViolation, ParseViolation},
		{"evt_", FormatEvent, ParseEvent},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			if id, err := tc.parse(tc.format(123)); id != 123 || err != nil || tc.format(123) != tc.prefix+"123" {
				t.Fatalf("round trip: %d %v", id, err)
			}
			for _, input := range []string{"", "wrong_1", tc.prefix, tc.prefix + "abc", tc.prefix + "0", tc.prefix + "-1", tc.prefix + "+1", tc.prefix + "1 ", tc.prefix + "9223372036854775808"} {
				if _, err := tc.parse(input); err == nil {
					t.Errorf("accepted %q", input)
				}
			}
		})
	}
}
