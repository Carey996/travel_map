// Package geo 提供地理计算。
package geo

import "math"

// EarthRadiusM 地球平均半径（米）。
const EarthRadiusM = 6371008.8

// Haversine 返回两点之间的球面距离（米）。
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dPhi := phi2 - phi1
	dLambda := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * EarthRadiusM * math.Asin(math.Sqrt(a))
}
