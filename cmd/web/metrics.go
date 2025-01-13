package main

import (
	"encoding/json"
	"expvar"
	"net/http"
	"sort"
)

func (app *application) metricsHandler(w http.ResponseWriter, r *http.Request) {
	type Metric struct {
		Key   string
		Value string
	}

	var metrics []Metric

	expvar.Do(func(kv expvar.KeyValue) {
		var value string
		switch v := kv.Value.(type) {
		case *expvar.Int:
			value = v.String()
		case *expvar.Float:
			value = v.String()
		case *expvar.String:
			value = v.String()
		case *expvar.Map:
			// For maps, convert to JSON for readable output
			mapData := make(map[string]interface{})
			v.Do(func(mapKV expvar.KeyValue) {
				mapData[mapKV.Key] = mapKV.Value.String()
			})
			if jsonData, err := json.MarshalIndent(mapData, "", "  "); err == nil {
				value = string(jsonData)
			} else {
				value = "error marshaling map"
			}
		default:
			value = v.String()
		}

		metrics = append(metrics, Metric{
			Key:   kv.Key,
			Value: value,
		})
	})

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Key < metrics[j].Key
	})

	templateData := app.newTemplateData(r)
	templateData["Metrics"] = metrics
	app.render(w, r, http.StatusOK, metricsPage, nil, templateData)
}
