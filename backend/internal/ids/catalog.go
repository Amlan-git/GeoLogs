package ids

import (
	"errors"
	"strconv"
	"strings"
)

func FormatVehicle(id int64) string         { return "veh_" + strconv.FormatInt(id, 10) }
func ParseVehicle(id string) (int64, error) { return parse(id, "veh_") }

func FormatGeofence(id int64) string         { return "geo_" + strconv.FormatInt(id, 10) }
func ParseGeofence(id string) (int64, error) { return parse(id, "geo_") }

func FormatAlertRule(id int64) string         { return "alert_" + strconv.FormatInt(id, 10) }
func ParseAlertRule(id string) (int64, error) { return parse(id, "alert_") }

func FormatViolation(id int64) string         { return "viol_" + strconv.FormatInt(id, 10) }
func ParseViolation(id string) (int64, error) { return parse(id, "viol_") }

func FormatEvent(id int64) string         { return "evt_" + strconv.FormatInt(id, 10) }
func ParseEvent(id string) (int64, error) { return parse(id, "evt_") }

var errInvalidID = errors.New("invalid resource ID")

func parse(value, prefix string) (int64, error) {
	digits, ok := strings.CutPrefix(value, prefix)
	if !ok {
		return 0, errInvalidID
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, errInvalidID
		}
	}
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidID
	}
	return id, nil
}
