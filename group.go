// Copyright 2021 Eurac Research. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package browser

import "strconv"

// Group combines multiple measurements to a single entity by
// predefined matching rules on the raw label names.
// The rules on how to match a raw label name to a group is
// defined by the GroupRegexpMap using regulare expressions.
type Group uint8

// GroupRegexpMap maps a Group to a regular expression for matching
// a raw label names of the measurements.
var GroupRegexpMap = map[Group]string{
	AirTemperature:                    "^air_t(.*)*$",
	RelativeHumidity:                  "^air_rh(.*)*$",
	SoilTemperature:                   "^st_.*|_st_.*",
	SoilWaterContent:                  "^swc_[^dp_|ec_|st_]",
	SoilElectricalConductivity:        "^swc_ec_",
	SoilDielectricPermittivity:        "^swc_dp_",
	SoilWaterPotential:                "^swp.[^_st_]",
	SoilHeatFlux:                      "^shf.*$",
	SoilSurfaceTemperature:            ".*surf_t.*$", // TODO: "surf_t_" and not("mv")
	WindSpeed:                         "^wind_speed.*$",
	WindDirection:                     "^wind_dir",
	Precipitation:                     "^precip.*(_tot|_int).*$",
	SnowHeight:                        "snow_height",
	LeafWetnessDuration:               "^lwm",
	SunshineDuration:                  "^sun",
	PhotosyntheticallyActiveRadiation: "^par_.*$",
	NDVIRadiations:                    "^ndvi_.*$",
	PRIRadiations:                     "^pri_.*$",
	ShortWaveRadiation:                "(^sr_|.*_sw_).*$",
	LongWaveRadiation:                 ".*_lw_.*$",
}

const (
	AirTemperature Group = iota
	RelativeHumidity
	SoilTemperature
	SoilWaterContent
	SoilElectricalConductivity
	SoilDielectricPermittivity
	SoilWaterPotential
	SoilHeatFlux
	SoilSurfaceTemperature
	WindSpeed
	WindDirection
	Precipitation
	SnowHeight
	LeafWetnessDuration
	SunshineDuration
	PhotosyntheticallyActiveRadiation
	NDVIRadiations
	PRIRadiations
	ShortWaveRadiation
	LongWaveRadiation

	// Sub Groups
	// TODO: is this working?
	Depth02
	Depth05
	Depth20
	Depth50
	Average
	Gust
	Total
	Intensity
	TotalIncoming
	DiffuseIncoming
	AtSoilLevelIncoming
	Incoming
	Outgoing
)

// DefaultGroups is a list containing the default groups.
var DefaultGroups = []Group{
	AirTemperature,
	RelativeHumidity,
	WindDirection,
	WindSpeed,
	ShortWaveRadiation,
	Precipitation,
	SnowHeight,
}

// AllGroups is a list of all supported groups.
var AllGroups = []Group{
	AirTemperature,
	RelativeHumidity,
	SoilTemperature,
	SoilWaterContent,
	SoilElectricalConductivity,
	SoilDielectricPermittivity,
	SoilWaterPotential,
	SoilHeatFlux,
	SoilSurfaceTemperature,
	WindSpeed,
	WindDirection,
	Precipitation,
	SnowHeight,
	LeafWetnessDuration,
	SunshineDuration,
	PhotosyntheticallyActiveRadiation,
	NDVIRadiations,
	PRIRadiations,
	ShortWaveRadiation,
	LongWaveRadiation,
}

func (g Group) String() string {
	switch g {
	default:
		return "No Group"
	case AirTemperature:
		return "Air Temperature"
	case RelativeHumidity:
		return "Relative Humidity"
	case SoilTemperature:
		return "Soil Temperature"
	case SoilWaterContent:
		return "Soil Water Content"
	case SoilElectricalConductivity:
		return "Soil Electrical Conductivity"
	case SoilDielectricPermittivity:
		return "Soil Dielectric Permittivity"
	case SoilWaterPotential:
		return "Soil Water Potential"
	case SoilHeatFlux:
		return "Soil Heat Flux"
	case SoilSurfaceTemperature:
		return "Soil Surface Temperature"
	case WindSpeed:
		return "Wind Speed"
	case WindDirection:
		return "Wind Direction"
	case Precipitation:
		return "Precipitation"
	case SnowHeight:
		return "Snow Height"
	case LeafWetnessDuration:
		return "Leaf Wetness Duration"
	case SunshineDuration:
		return "Sunshine Duration"
	case PhotosyntheticallyActiveRadiation:
		return "Photosynthetically Active Radiation"
	case NDVIRadiations:
		return "NDVI Radiations"
	case PRIRadiations:
		return "PRI Radiations"
	case ShortWaveRadiation:
		return "Short Wave Radiation"
	case LongWaveRadiation:
		return "Long Wave Radiation"
	case Depth02:
		return "Depth: 2 cm"
	case Depth05:
		return "Depth: 5 cm"
	case Depth20:
		return "Depth: 20 cm"
	case Depth50:
		return "Depth: 50 cm"
	case Average:
		return "Average"
	case Gust:
		return "Gust"
	case Total:
		return "Total"
	case Intensity:
		return "Intensity"
	case TotalIncoming:
		return "Total Incoming"
	case DiffuseIncoming:
		return "Diffuse Incoming"
	case AtSoilLevelIncoming:
		return "At Soil Level Incoming"
	case Incoming:
		return "Incoming"
	case Outgoing:
		return "Outgoing"
	}
}

func (g Group) Sub() []Group {
	switch g {
	default:
		return nil

	case SoilTemperature | SoilElectricalConductivity:
		return []Group{Depth02, Depth05, Depth20, Depth50}

	case WindSpeed:
		return []Group{Average, Gust}

	case Precipitation:
		return []Group{Total, Intensity}

	case PhotosyntheticallyActiveRadiation:
		return []Group{TotalIncoming, DiffuseIncoming, AtSoilLevelIncoming}

	case ShortWaveRadiation | LongWaveRadiation:
		return []Group{Incoming, Outgoing}

	}
}

// ParseGroup parses the given string or string slice into a slice of
// Group and will remove duplicates.
func ParseGroup(str ...string) []Group {
	var g []Group

	for _, s := range str {
		i, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			continue
		}

		g = uniqueGroup(g, Group(i))
	}

	return g
}

func uniqueGroup(slice []Group, g Group) []Group {
	for _, el := range slice {
		if el == g {
			return slice
		}
	}
	return append(slice, g)
}
