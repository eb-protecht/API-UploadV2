package models

type Location struct {
	Type        string    `json:"type" bson:"type"`
	Coordinates []float64 `json:"coordinates" bson:"coordinates"`
}

// NewPoint returns a GeoJSON Point with longitude and latitude.
func NewStreamLocation(long, lat float64) Location {
	return Location{
		"Point",
		[]float64{long, lat},
	}
}
