package routing

import (
	"errors"
	"math"
	"strings"

	"josex/web/modules/geo/models"
)

// Valhalla encodes shapes as Google polylines at precision 6, one per leg. The trip is the legs end
// to end, each starting where the previous one stopped, so joining is decode, drop the repeated
// joint, encode.

var errBadPolyline = errors.New("routing: malformed polyline")

func joinPolylines(shapes []string) (string, error) {
	if len(shapes) == 1 {
		return shapes[0], nil
	}
	var points [][2]int64
	for _, shape := range shapes {
		leg, err := decodePolyline(shape)
		if err != nil {
			return "", err
		}
		if len(leg) > 0 && len(points) > 0 && leg[0] == points[len(points)-1] {
			leg = leg[1:]
		}
		points = append(points, leg...)
	}
	return encodePolyline(points), nil
}

// decodePolyline returns [lat, lng] pairs as the encoded integers (degrees × 1e6).
func decodePolyline(s string) ([][2]int64, error) {
	var points [][2]int64
	var at [2]int64
	for i := 0; i < len(s); {
		for axis := range at {
			var result int64
			for shift := uint(0); ; shift += 5 {
				if i >= len(s) {
					return nil, errBadPolyline
				}
				b := int64(s[i]) - 63
				i++
				result |= (b & 0x1f) << shift
				if b < 0x20 {
					break
				}
			}
			if result&1 != 0 {
				at[axis] += ^(result >> 1)
			} else {
				at[axis] += result >> 1
			}
		}
		points = append(points, at)
	}
	return points, nil
}

func encodePolyline(points [][2]int64) string {
	var b strings.Builder
	var prev [2]int64
	for _, p := range points {
		for axis := range p {
			delta := p[axis] - prev[axis]
			v := delta << 1
			if delta < 0 {
				v = ^v
			}
			for v >= 0x20 {
				b.WriteByte(byte((0x20 | (v & 0x1f)) + 63))
				v >>= 5
			}
			b.WriteByte(byte(v + 63))
		}
		prev = p
	}
	return b.String()
}

// EncodePolyline6 encodes points at precision 6, as Valhalla does, for a line that did not come from
// Valhalla: a raw GPS path while the router is down.
func EncodePolyline6(points []models.LatLng) string {
	encoded := make([][2]int64, len(points))
	for i, p := range points {
		encoded[i] = [2]int64{int64(math.Round(p.Lat * 1e6)), int64(math.Round(p.Lng * 1e6))}
	}
	return encodePolyline(encoded)
}

// LineLengthM is the length of a path in metres, point to point on the sphere.
func LineLengthM(points []models.LatLng) float64 {
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += haversineM(points[i-1], points[i])
	}
	return total
}

// lineLengthM measures a decoded polyline (degrees × 1e6).
func lineLengthM(points [][2]int64) float64 {
	line := make([]models.LatLng, len(points))
	for i, p := range points {
		line[i] = models.LatLng{Lat: float64(p[0]) / 1e6, Lng: float64(p[1]) / 1e6}
	}
	return LineLengthM(line)
}

func haversineM(a, b models.LatLng) float64 {
	const earthRadiusM = 6371000.0
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLng := (b.Lng - a.Lng) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}
