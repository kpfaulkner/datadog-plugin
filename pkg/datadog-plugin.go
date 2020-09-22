package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type DatadogQuery struct {
	Constant      float64 `json:"constant"`
	Datasource    string  `json:"datasource"`
	DatasourceID  int     `json:"datasourceId"`
	IntervalMs    int     `json:"intervalMs"`
	MaxDataPoints int     `json:"maxDataPoints"`
	OrgID         int     `json:"orgId"`
	QueryText     string  `json:"queryText"`
	RefID         string  `json:"refId"`
}

type DatadogPluginConfig struct {
	DatadogAPIKey string `json:"datadogApiKey"`
	DatadogAppKey string `json:"datadogAppKey"`
}

// newDatasource returns datasource.ServeOpts.
func newDatadogDataSource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newDataSourceInstance)
	ds := &DatadogDataSource{
		im: im,
	}

	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

// DatadogDataSource.... all things DD :)
type DatadogDataSource struct {
	// The instance manager can help with lifecycle management
	// of datasource instances in plugins. It's not a requirements
	// but a best practice that we recommend that you follow.
	im instancemgmt.InstanceManager

	// creds to DD
	datadogApiKey string
	datadogAppKey string
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (td *DatadogDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData", "request", req)

	queryBytes, _ := req.Queries[0].JSON.MarshalJSON()
	var query DatadogQuery
	err := json.Unmarshal(queryBytes, &query)
	if err != nil {
		return nil, err
	}

	configBytes, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()
	var config DatadogPluginConfig
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}

	configString := fmt.Sprintf("CONFIG  %v", config)
	queryString := fmt.Sprintf("QUERY %v", query)
	log.DefaultLogger.Info(configString)
	log.DefaultLogger.Info(queryString)

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := td.query(ctx, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	Format string `json:"format"`
}

func (td *DatadogDataSource) query(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	// Unmarshal the json into our queryModel
	var qm queryModel

	response := backend.DataResponse{}

	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return response
	}

	// Log a warning if `Format` is empty.
	if qm.Format == "" {
		log.DefaultLogger.Warn("format is empty. defaulting to time series")
	}

	// create data frame response
	frame := data.NewFrame("response")

	// add the time dimension
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{query.TimeRange.From, query.TimeRange.From.Add(10 * time.Minute), query.TimeRange.To}),
	)

	// add values
	frame.Fields = append(frame.Fields,
		data.NewField("stuff", nil, []int64{10, 20, 15}),
	)

	// add the frames to the response
	response.Frames = append(response.Frames, frame)

	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (td *DatadogDataSource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {

	rawJson, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()

	v := fmt.Sprintf("ZZZZZZZZZZZ heath config %s", string(rawJson))
	log.DefaultLogger.Info(v)

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	if rand.Int()%2 == 0 {
		status = backend.HealthStatusError
		message = "randomized error"
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

type instanceSettings struct {
	httpClient *http.Client
}

func newDataSourceInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {

	log.DefaultLogger.Info("YYYYYYYYYYYYYYYYYYYYYYYYYYYYYY")
	log.DefaultLogger.Info("settings", "settings", setting)

	fmt.Printf("settings %v\n", setting)
	return &instanceSettings{
		httpClient: &http.Client{},
	}, nil
}

func (s *instanceSettings) Dispose() {
	// Called before creatinga a new instance to allow plugin authors
	// to cleanup.
}
