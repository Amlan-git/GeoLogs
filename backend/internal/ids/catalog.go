package ids

import "strconv"

func FormatGeofence(id int64) string {
	return "geo_" + strconv.FormatInt(id, 10)
}

func FormatVehicle(id int64) string {
	return "veh_" + strconv.FormatInt(id, 10)
}
