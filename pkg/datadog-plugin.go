package main

import (
  "context"
  "encoding/json"
  "fmt"
  "github.com/kpfaulkner/ddlog/pkg/models"
  "net/http"
  "strings"
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

type queryModel struct {
	Format string `json:"format"`
}

// newDatasource returns datasource.ServeOpts.
func newDatadogDataSource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newDataSourceInstance)
	ds := &DatadogDataSource{
		im:    im,
		cache: NewSimpleCache(),
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
	cache        *SimpleCache
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (td *DatadogDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// Haven't created Datadog instance yet (no API keys yet).
	// So do this now!
	// Need to check if this is threadsafe
	if td.datadogComms == nil {

		log.DefaultLogger.Info("setting up DD comms!!!!")
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

// executeQuery
func (td *DatadogDataSource) executeQuery(queryText string, fromDate time.Time, toDate time.Time) ([]models.DataDogLog, error) {
	resp, err := td.datadogComms.QueryDatadog(queryText, fromDate.UTC(), toDate.UTC())
	if err != nil {
		return nil, err
	}

	logs := []models.DataDogLog{}
	logs = append(logs, resp.Logs...)

	// now loop until no nextId
	for resp.NextLogID != "" {
		resp, err = td.datadogComms.QueryDatadogWithStartAt(queryText, fromDate.UTC(), toDate.UTC(), resp.NextLogID)
		if err != nil {
			fmt.Printf("ERROR %s\n", err.Error())
			return nil, err
		}
		logs = append(logs, resp.Logs...)
	}

	return logs, nil
}

// checkCache checks the cache for the correct query. If there is any time overlap between cache entry
// and new query, then only actually query DD for time range NOT in cache. Man that could be worded better.
func (td *DatadogDataSource) checkCache(query string, startTime time.Time) (*time.Time, error) {

  v := fmt.Sprintf("checkCache with start%s", startTime.UTC().Format("2006-01-02 15:04:05"))
  log.DefaultLogger.Info(v)

	lowerQuery := strings.ToLower(query)
	cacheEntry, ok := td.cache.Get(lowerQuery)
	if ok {
    v := fmt.Sprintf("cache hit for query %s : st %s  : et %s", lowerQuery, cacheEntry.StartTime.UTC().Format("2006-01-02 15:04:05"), cacheEntry.EndTime.UTC().Format("2006-01-02 15:04:05"))
    log.DefaultLogger.Info(v)
		// have results in cache. Check if there is any overlap with new query and cache entry.
		if cacheEntry.EndTime.After(startTime) && startTime.After(cacheEntry.StartTime){
		  //&&    } cacheEntry.StartTime.Before(startTime){

		  // return 2 minutes earlier...  just to avoid any issues picking
		  // Also round to beginning of minute.
		  roundedTime := time.Date(cacheEntry.EndTime.Year(), cacheEntry.EndTime.Month(),
        cacheEntry.EndTime.Day(), cacheEntry.EndTime.Hour(),
        cacheEntry.EndTime.Add(-2*time.Minute).Minute(), 0, 0, cacheEntry.EndTime.Location())

			return &roundedTime, nil
		}
	}
	return &startTime, nil
}

func (td *DatadogDataSource) addToAndReturnCache(logs []models.DataDogLog, query string, startTime time.Time) (*CacheEntry, error) {

  var ce *CacheEntry
  var ok bool
  lowerQuery := strings.ToLower(query)

  // merge these logs into cache.
  ce, ok = td.cache.Get(lowerQuery)
  if !ok {
    // dont have one... create a new one.
    ce = NewCacheEntry()
    ce.Query = lowerQuery
  }

  // add all new entries to cache.
  for _, logEntry := range logs {

    roundedTime := time.Date(logEntry.Content.Timestamp.Year(), logEntry.Content.Timestamp.Month(),
      logEntry.Content.Timestamp.Day(), logEntry.Content.Timestamp.Hour(),
      logEntry.Content.Timestamp.Minute(), 0, 0, logEntry.Content.Timestamp.Location())

    // increment one at a time...
    ce.Data[roundedTime] = ce.Data[roundedTime] + 1
  }

  // prune old entries.
  count, _ := ce.PruneBefore(startTime.UTC())

  // save it.
  td.cache.Set(lowerQuery, ce)
  return ce,nil
}

// 1) check if query exists in cache.
// 2) if query exists, check if cache covers time range required
// 3) if does have time range, then just return data from query
// 4) if cache does NOT have appropriate time range then do query and populate cache
//    If the cache covers PART of the time range required, then only query DD for the missing parts
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

	response := backend.DataResponse{}
	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return nil, err
	}

	// Log a warning if `Format` is empty.
	if qm.Format == "" {
		log.DefaultLogger.Warn("format is empty. defaulting to time series")
	}

  v := fmt.Sprintf("ORIG start time %s end time %s", query.TimeRange.From.UTC().Format("2006-01-02 15:04:05"),query.TimeRange.To.UTC().Format("2006-01-02 15:04:05"))
  log.DefaultLogger.Info(v)

  // check cache for if we should modify the query.
	newStartTime, err := td.checkCache(ddQuery.QueryText, query.TimeRange.From.UTC())
	if err != nil {
    return nil, err
  }

  v = fmt.Sprintf("start time %s end time %s", newStartTime.UTC().Format("2006-01-02 15:04:05"),query.TimeRange.To.UTC().Format("2006-01-02 15:04:05"))
  log.DefaultLogger.Info(v)

	logs, err := td.executeQuery(ddQuery.QueryText, newStartTime.UTC(), query.TimeRange.To.UTC())
	if err != nil {
		return nil, err
	}

	// create data frame response
	frame := data.NewFrame("response")

	groupedLogs := ddlog.GroupLogsByMinute(logs)

	var ce *CacheEntry
	var ok bool
  lowerQuery := strings.ToLower(ddQuery.QueryText)
	// merge these logs into cache.
	ce, ok = td.cache.Get(lowerQuery)
	if !ok {
	  // dont have one... create a new one.
    ce = NewCacheEntry()
    ce.Query = lowerQuery
  }

  v = fmt.Sprintf("number of new logs %d", len(logs))
  log.DefaultLogger.Info(v)

	// add all new entries to cache.
	for _, logEntry := range logs {

		roundedTime := time.Date(logEntry.Content.Timestamp.Year(), logEntry.Content.Timestamp.Month(),
			logEntry.Content.Timestamp.Day(), logEntry.Content.Timestamp.Hour(),
			logEntry.Content.Timestamp.Minute(), 0, 0, logEntry.Content.Timestamp.Location())

		numberOfEntries := len(groupedLogs[roundedTime])
		ce.Data[roundedTime] = int64(numberOfEntries)
	}


	//ce.StartTime = newStartTime.UTC()
  ce.StartTime = query.TimeRange.From.UTC()
	ce.EndTime = query.TimeRange.To.UTC()

  // prune old entries.
  ce.PruneBefore(ce.StartTime)

  // save it.
	td.cache.Set(lowerQuery, ce)

	// get linear list of logs from cache.
	times := ce.GetKeysInOrder()

  // generate time slice.
  counts := []int64{}

  v = fmt.Sprintf("number of TOTAL unique times logs %d", len(times))
  log.DefaultLogger.Info(v)

  for _,t := range times {
    c := ce.Data[t]
    counts = append(counts, c)
  }

	// add the time dimension
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, times),
	)

	// add values
	frame.Fields = append(frame.Fields,
		data.NewField("entries", nil, counts),
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

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	rawJson, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()
	var config DatadogPluginConfig
	err := json.Unmarshal(rawJson, &config)
	if err != nil {
		status = backend.HealthStatusError
		message = "Unable to communicate with Datadog"
	}

	// just do query ... no specific query but time range is same.
	// If there is an auth issue the resp.Status will be "error"\
	// WHY the err isn't set, I dont know.
	td.datadogComms = ddlog.NewDatadog(config.DatadogAPIKey, config.DatadogAppKey)
	resp, err := td.datadogComms.QueryDatadog("", time.Now().UTC(), time.Now().UTC())
	if err != nil || resp.Status == "error" {
		status = backend.HealthStatusError
		message = "Unable to communicate with Datadog"
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
	return &instanceSettings{
		httpClient: &http.Client{},
	}, nil
}

func (s *instanceSettings) Dispose() {
	// Called before creatinga a new instance to allow plugin authors
	// to cleanup.
}
