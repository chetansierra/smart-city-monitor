package zones

// ZoneBounds defines the geographic boundaries of a zone.
type ZoneBounds struct {
	MinLat float64
	MaxLat float64
	MinLng float64
	MaxLng float64
}

// Zones defines the geographic zones for the smart city (NYC-based).
var Zones = map[string]ZoneBounds{
	"downtown": {MinLat: 40.700, MaxLat: 40.725, MinLng: -74.020, MaxLng: -74.000},
	"midtown":  {MinLat: 40.725, MaxLat: 40.765, MinLng: -74.000, MaxLng: -73.970},
	"uptown":   {MinLat: 40.765, MaxLat: 40.800, MinLng: -73.990, MaxLng: -73.955},
}

// GetZone returns the zone name for a given lat/lng. Returns "unknown" if not in any zone.
func GetZone(lat, lng float64) string {
	for name, bounds := range Zones {
		if lat >= bounds.MinLat && lat <= bounds.MaxLat &&
			lng >= bounds.MinLng && lng <= bounds.MaxLng {
			return name
		}
	}
	return "unknown"
}

// GetZoneFromKey returns a zone string from a key (used for partitioning).
// Since we don't have lat/lng from just a key, this falls back to the key itself
// for consistent hashing. Actual zone detection uses GetZone with coordinates.
func GetZoneFromKey(key string) string {
	return key
}

// AllZoneNames returns all defined zone names.
func AllZoneNames() []string {
	names := make([]string, 0, len(Zones))
	for name := range Zones {
		names = append(names, name)
	}
	return names
}
