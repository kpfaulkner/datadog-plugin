# Datadog-plugin for Grafana

This is an initial attempt for retrieving log counts from Datadog for use in Grafana. Very early stages, but DOES work.

![datadog-grafana](./images/datadog-grafana.png)


## Description

The current (initial) implementation simply performs counts on returned logs. Ideally this 
will change to some statistics API within Datadog (but no luck yet and this simple approach
currently in use is good enough for my immediate needs)

## Configuration

A Datadog API key and App Key are required for integration. See [here](https://docs.datadoghq.com/account_management/api-app-keys/#application-keys) for details.

When configuring the query within Grafana, remember if you perform a query that is *excessive* (eg, return every log for the last year), you're going to not have an optimal experience. I strongly suggest testing out the queries first in Datadog (which you probably would anyway while authoring them). 



## Installation

Currently this plugin is NOT signed (Grafana are still determining the best approach for having 3rd party plugins signed), so if you want to run this plugin there are couple of steps required

- Modify the Grafana defaults.ini file, in the [paths] section and include a line similar to: *plugins = "C:\temp\grafana-plugins"*  (or whichever path you install this plugin)
- Also modify defaults.ini (in [plugins] section) and modify to have a line similar to: *allow_loading_unsigned_plugins = kpfaulkner-datadog-plugin*  . This will allow a non-signed plugin to be executed. See [the docs](https://grafana.com/docs/grafana/latest/administration/configuration/#allow-loading-unsigned-plugins) for details. (note: you don't have to modify default.ini but can also choose other .ini files)
- For *nix systems, make sure the generated executable (gpx_datadog-plugin*) has the executable file flag set.





