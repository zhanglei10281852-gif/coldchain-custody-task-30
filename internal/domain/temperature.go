package domain

import (
	"fmt"
	"math"
)

// MilliCelsius stores temperatures without floating-point persistence drift.
type MilliCelsius int64

func TemperatureFromCelsius(value float64) (MilliCelsius, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, FieldError{Field: "temperature", Message: "must be finite"}
	}
	if value < -196 || value > 100 {
		return 0, FieldError{Field: "temperature", Message: "outside supported range"}
	}
	return MilliCelsius(math.Round(value * 1000)), nil
}

func (t MilliCelsius) Celsius() float64 { return float64(t) / 1000 }

func (t MilliCelsius) String() string { return fmt.Sprintf("%.3fC", t.Celsius()) }

type TemperatureRange struct {
	Minimum MilliCelsius `json:"minimum"`
	Maximum MilliCelsius `json:"maximum"`
}

func NewTemperatureRange(minimum, maximum MilliCelsius) (TemperatureRange, error) {
	if minimum >= maximum {
		return TemperatureRange{}, FieldError{Field: "temperature_range", Message: "minimum must be lower than maximum"}
	}
	return TemperatureRange{Minimum: minimum, Maximum: maximum}, nil
}

func (r TemperatureRange) Contains(value MilliCelsius) bool {
	return value >= r.Minimum && value <= r.Maximum
}

func (r TemperatureRange) Validate() error {
	if r.Minimum >= r.Maximum {
		return FieldError{Field: "temperature_range", Message: "minimum must be lower than maximum"}
	}
	return nil
}
