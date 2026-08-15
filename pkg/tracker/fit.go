package tracker

import (
	"os"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"tour-map/pkg/geo"
)

// ParseFitFile parses a Garmin/FIT activity file and extracts timestamped GPS waypoints.
func ParseFitFile(path string) ([]geo.Waypoint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := decoder.New(file)
	var waypoints []geo.Waypoint

	for dec.Next() {
		fitFile, err := dec.Decode()
		if err != nil {
			return nil, err
		}

		for i := range fitFile.Messages {
			if fitFile.Messages[i].Num != mesgnum.Record {
				continue
			}

			record := mesgdef.NewRecord(&fitFile.Messages[i])
			if record.PositionLat != basetype.Sint32Invalid && record.PositionLong != basetype.Sint32Invalid {
				lat := record.PositionLatDegrees()
				lng := record.PositionLongDegrees()

				waypoints = append(waypoints, geo.Waypoint{
					Location: &geo.GPSCoords{
						Latitude:  lat,
						Longitude: lng,
					},
					Timestamp: record.Timestamp,
				})
			}
		}
	}

	return waypoints, nil
}
