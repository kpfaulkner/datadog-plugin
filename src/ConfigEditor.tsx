import React, { ChangeEvent, PureComponent } from 'react';
import { LegacyForms } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions } from './types';

const { FormField } = LegacyForms;

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  onApiKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      datadogApiKey: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onAppKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      datadogAppKey: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onResetAPIKey = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        apiKey: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        apiKey: '',
      },
    });
  };

  render() {
    const { options } = this.props;
    const { jsonData } = options;
    //const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;

    return (
      <div className="gf-form-group">
        <div className="gf-form">
          <FormField
            label="DataDogApiKey"
            labelWidth={6}
            inputWidth={20}
            onChange={this.onApiKeyChange}
            value={jsonData.datadogApiKey || ''}
            placeholder="Datadog API Key"
          />

        </div>
        <div className="gf-form">
        <FormField
          label="DataDogAppKey"
          labelWidth={6}
          inputWidth={20}
          onChange={this.onAppKeyChange}
          value={jsonData.datadogAppKey|| ''}
          placeholder="Datadog App Key"
        />
        </div>
      </div>
    );
  }
}
