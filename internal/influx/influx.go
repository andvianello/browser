// Copyright 2020 Eurac Research. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package influx provides the implementation of the browser.Database interface
// using InfluxDB as backend.
package influx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/euracresearch/browser"
	"github.com/euracresearch/browser/internal/ql"

	client "github.com/influxdata/influxdb1-client/v2"
)

// Guarantee we implement browser.Series.
var _ browser.Database = &DB{}

// DB holds information for communicating with InfluxDB.
type DB struct {
	client   client.Client
	database string

	mu    sync.RWMutex
	cache map[int64][]browser.Group
}

// NewDB returns a new instance of DB.
func NewDB(client client.Client, database string) *DB {

	return &DB{
		client:   client,
		database: database,
	}
}

// measurementsFromGroup returns a list of measurements for each given group.
// Dublicates will be removed.
func (db *DB) measurementsFromGroup(groups []browser.Group, showSTD bool) []string {
	var measure []string
	for _, group := range groups {
		re, ok := browser.GroupRegexpMap[group]
		if !ok {
			continue
		}

		q := fmt.Sprintf("SHOW MEASUREMENTS WITH MEASUREMENT =~ /%s/", re)
		resp, err := db.client.Query(client.NewQuery(q, db.database, ""))
		if err != nil {
			log.Println(err)
			continue
		}
		if resp.Error() != nil {
			log.Println(err)
			continue
		}

		for _, result := range resp.Results {
			for _, serie := range result.Series {
				for _, value := range serie.Values {
					for _, v := range value {
						m := v.(string)
						if strings.HasSuffix(m, "_std") && !showSTD {
							continue
						}

						measure = browser.Unique(measure, m)
					}
				}
			}
		}

	}

	return measure
}

func (db *DB) GroupsByStation(ctx context.Context, id int64) ([]browser.Group, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	g, ok := db.cache[id]
	if ok {
		db.mu.RUnlock()
		return g, nil
	}

	// get measurements from db

	return nil, nil
}

// Series return a browser.TimeSeries from the given message.
func (db *DB) Series(ctx context.Context, m *browser.Message) (browser.TimeSeries, error) {
	if m == nil {
		return nil, browser.ErrDataNotFound
	}

	resp, err := db.exec(db.seriesQuery(m))
	if err != nil {
		return nil, err
	}

	var ts browser.TimeSeries
	for _, result := range resp.Results {
		for _, serie := range result.Series {
			nTime := m.Start

			m := &browser.Measurement{
				Label:       serie.Name,
				Station:     serie.Tags["station"],
				Landuse:     serie.Tags["landuse"],
				Aggregation: serie.Tags["aggr"],
				Unit:        serie.Tags["unit"],
			}

			for _, value := range serie.Values {
				t, err := time.ParseInLocation(time.RFC3339, value[0].(string), time.UTC)
				if err != nil {
					log.Printf("cannot convert timestamp: %v. skipping.", err)
					continue
				}

				// Fill missing timestamps with NaN values, to return a time
				// series with a continuous time range. The interval of raw data
				// in LTER is 15 minutes. See:
				// https://github.com/euracresearch/browser/issues/10
				for !t.Equal(nTime) {
					m.Points = append(m.Points, &browser.Point{
						Timestamp: nTime,
						Value:     math.NaN(),
					})
					nTime = nTime.Add(browser.DefaultCollectionInterval)
				}
				nTime = t.Add(browser.DefaultCollectionInterval)

				f, err := value[1].(json.Number).Float64()
				if err != nil {
					log.Printf("cannot convert value to float: %v. skipping.", err)
					continue
				}

				// Add additional metadata only on the first run.
				m.Elevation, err = value[2].(json.Number).Int64()
				if err != nil {
					m.Elevation = -1
				}

				m.Latitude, err = value[3].(json.Number).Float64()
				if err != nil {
					m.Latitude = -1.0
				}

				m.Longitude, err = value[4].(json.Number).Float64()
				if err != nil {
					m.Longitude = -1.0
				}

				if value[5] == nil {
					m.Depth = 0
				} else {
					m.Depth, err = value[5].(json.Number).Int64()
					if err != nil {
						m.Depth = -1
					}
				}
				p := &browser.Point{
					Timestamp: t,
					Value:     f,
				}
				m.Points = append(m.Points, p)
			}

			ts = append(ts, m)
		}
	}

	return ts, nil
}

func (db *DB) seriesQuery(m *browser.Message) ql.Querier {
	return ql.QueryFunc(func() (string, []interface{}) {
		var (
			buf  bytes.Buffer
			args []interface{}
		)

		// Data in InfluxDB is UTC but LTER data is UTC+1 therefor we need to adapt
		// start and end times. It will shift the start time to -1 hour and will set
		// the end time to 22:59:59 in order to capture a full day.
		start := m.Start.Add(-1 * time.Hour)
		end := time.Date(m.End.Year(), m.End.Month(), m.End.Day(), 22, 59, 59, 59, time.UTC)

		for _, measure := range db.measurementsFromGroup(m.Measurements, m.ShowSTD) {
			columns := []string{measure, "altitude as elevation", "latitude", "longitude", "depth"}

			sb := ql.Select(columns...)
			sb.From(measure)
			sb.Where(
				ql.Eq(ql.Or(), "snipeit_location_ref", m.Stations...),
				ql.And(),
				ql.TimeRange(start, end),
			)
			sb.GroupBy("station,snipeit_location_ref,landuse,unit,aggr")
			sb.OrderBy("time").ASC().TZ("Etc/GMT-1")

			q, arg := sb.Query()
			buf.WriteString(q)
			buf.WriteString(";")

			args = append(args, arg)
		}

		return buf.String(), args
	})
}

func (db *DB) Query(ctx context.Context, m *browser.Message) *browser.Stmt {
	c := []string{"station", "landuse", "altitude as elevation", "latitude", "longitude"}
	measures := db.measurementsFromGroup(m.Measurements, m.ShowSTD)
	c = append(c, measures...)

	// Data in InfluxDB is UTC but LTER data is UTC+1 therefor we need to adapt
	// start and end times. It will shift the start time to -1 hour and will set
	// the end time to 22:59:59 in order to capture a full day.
	start := m.Start.Add(-1 * time.Hour)
	end := time.Date(m.End.Year(), m.End.Month(), m.End.Day(), 22, 59, 59, 59, time.UTC)

	q, _ := ql.Select(c...).From(measures...).Where(
		ql.Eq(ql.Or(), "snipeit_location_ref", m.Stations...),
		ql.And(),
		ql.TimeRange(start, end),
	).OrderBy("time").ASC().TZ("Etc/GMT-1").Query()

	return &browser.Stmt{
		Query:    q,
		Database: db.database,
	}
}

// exec executes the given ql query and returns a response.
func (db *DB) exec(q ql.Querier) (*client.Response, error) {
	query, _ := q.Query()

	log.Println(query)
	resp, err := db.client.Query(client.NewQuery(query, db.database, ""))
	if err != nil {
		return nil, err
	}
	if resp.Error() != nil {
		return nil, fmt.Errorf("%v", resp.Error())
	}

	return resp, nil
}
