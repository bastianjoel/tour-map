package tracker

import (
	"os"
	"path/filepath"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"tour-map/pkg/geo"
)

// ParseFitFile parses a Garmin/FIT activity file and extracts timestamped GPS waypoints.
// When event start-stop timestamps (or session start/stop times) are present in the FIT file,
// they define the activity's date and timeframe rather than file course creation dates.
func ParseFitFile(path string) ([]geo.Waypoint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := decoder.New(file)
	var rawWaypoints []geo.Waypoint
	activityID := "fit:" + filepath.Base(path)

	var eventStart time.Time
	var eventStop time.Time

	for dec.Next() {
		fitFile, err := dec.Decode()
		if err != nil {
			return nil, err
		}

		for i := range fitFile.Messages {
			msg := &fitFile.Messages[i]

			switch msg.Num {
			case mesgnum.Record:
				record := mesgdef.NewRecord(msg)
				if record.PositionLat != basetype.Sint32Invalid && record.PositionLong != basetype.Sint32Invalid {
					lat := record.PositionLatDegrees()
					lng := record.PositionLongDegrees()

					rawWaypoints = append(rawWaypoints, geo.Waypoint{
						Location: &geo.GPSCoords{
							Latitude:  lat,
							Longitude: lng,
						},
						Timestamp:  record.Timestamp,
						ActivityID: activityID,
					})
				}

			case mesgnum.Event:
				event := mesgdef.NewEvent(msg)
				ts := event.Timestamp
				if ts.IsZero() && !event.StartTimestamp.IsZero() {
					ts = event.StartTimestamp
				}
				if !ts.IsZero() {
					switch event.EventType {
					case typedef.EventTypeStart:
						if eventStart.IsZero() || ts.Before(eventStart) {
							eventStart = ts
						}
					case typedef.EventTypeStop, typedef.EventTypeStopAll, typedef.EventTypeStopDisable, typedef.EventTypeStopDisableAll:
						if eventStop.IsZero() || ts.After(eventStop) {
							eventStop = ts
						}
					}
				}

			case mesgnum.Session:
				session := mesgdef.NewSession(msg)
				if !session.StartTime.IsZero() && (eventStart.IsZero() || session.StartTime.Before(eventStart)) {
					eventStart = session.StartTime
				}
				if !session.Timestamp.IsZero() && (eventStop.IsZero() || session.Timestamp.After(eventStop)) {
					eventStop = session.Timestamp
				}

			case mesgnum.Activity:
				activity := mesgdef.NewActivity(msg)
				if !activity.Timestamp.IsZero() && (eventStop.IsZero() || activity.Timestamp.After(eventStop)) {
					eventStop = activity.Timestamp
				}
			}
		}
	}

	if len(rawWaypoints) == 0 {
		return rawWaypoints, nil
	}

	// If eventStart is specified and differs from the initial waypoint timestamp (e.g. course date vs event date),
	// shift waypoints so the tour aligns with the actual event start-stop range.
	if !eventStart.IsZero() {
		offset := eventStart.Sub(rawWaypoints[0].Timestamp)
		if offset != 0 {
			for i := range rawWaypoints {
				rawWaypoints[i].Timestamp = rawWaypoints[i].Timestamp.Add(offset)
			}
		}
		// If eventStop is defined and is after the last waypoint, adjust the last waypoint timestamp
		if !eventStop.IsZero() && rawWaypoints[len(rawWaypoints)-1].Timestamp.Before(eventStop) {
			rawWaypoints[len(rawWaypoints)-1].Timestamp = eventStop
		}
	}

	return rawWaypoints, nil
}
