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
	ddlog "github.com/kpfaulkner/ddlog/pkg"
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

	datadogComms *ddlog.Datadog
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (td *DatadogDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData", "request", req)

	// Haven't created Datadog instance yet (no API keys yet).
	// So do this now!
	// Need to check if this is threadsafe
	if td.datadogComms == nil {
    configBytes, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()
    var config DatadogPluginConfig
    err := json.Unmarshal(configBytes, &config)
    if err != nil {
      return nil, err
    }
    td.datadogComms = ddlog.NewDatadog(config.DatadogAPIKey, config.DatadogAppKey)
  }



	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res, err := td.query(ctx, q)
		if err != nil {
		  return nil, err
    }

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = *res
	}

	return response, nil
}

type queryModel struct {
	Format string `json:"format"`
}

func (td *DatadogDataSource) query(ctx context.Context, query backend.DataQuery) (*backend.DataResponse, error) {
	// Unmarshal the json into our queryModel
	var qm queryModel

  queryBytes, _ := query.JSON.MarshalJSON()
  var ddQuery DatadogQuery
  err := json.Unmarshal(queryBytes, &ddQuery)
  if err != nil {
    // empty response? or real error? figure out later.
    return nil, err
  }


  v := fmt.Sprintf("QUERY!!! %d", ddQuery.IntervalMs)
  log.DefaultLogger.Info(v)

	response := backend.DataResponse{}
	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return nil, err
	}

	// Log a warning if `Format` is empty.
	if qm.Format == "" {
		log.DefaultLogger.Warn("format is empty. defaulting to time series")
	}

	// need to validate query to some degree. TODO(kpfaulkner) Validate!
	queryText := ddQuery.QueryText
  ddResponse, err := td.datadogComms.QueryDatadog(queryText, query.TimeRange.From.UTC(), query.TimeRange.To.UTC())
  if err != nil {
    return nil, err
  }

	// create data frame response
	frame := data.NewFrame("response")

	// generate time slice.
	times := []time.Time{}
  //counts := []int{}

  //query.
  for _,res := range ddResponse.Logs {
	  times = append(times, res.Content.Timestamp)
	  //counts = append(counts, res.Content.Message)
  }

  // counts

	// add the time dimension
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{query.TimeRange.From.UTC(), query.TimeRange.From.Add(10 * time.Minute), query.TimeRange.To}),
	)

	// add values
	frame.Fields = append(frame.Fields,
		data.NewField("stuff", nil, []int64{10, 20, 15}),
	)

	// add the frames to the response
	response.Frames = append(response.Frames, frame)

	return &response, nil
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
