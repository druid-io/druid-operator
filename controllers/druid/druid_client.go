package druid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type DruidClient struct {
	BaseUrl  string
	UserName string
	PassWord string
	client   *http.Client
}

func InitClient(BaseUrl string, UserName string, PassWord string) DruidClient {
	d := DruidClient{BaseUrl, UserName, PassWord, &http.Client{}}
	return d
}

func (d DruidClient) GetCoordinatorConfig() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", d.BaseUrl+"/druid/coordinator/v1/config", nil)
	req.SetBasicAuth(d.UserName, d.PassWord)
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	bodyText, err := io.ReadAll(resp.Body)
	s := string(bodyText)
	var m map[string]interface{}
	json.Unmarshal([]byte(s), &m)
	return m, nil
}

func (d DruidClient) UpdateCoordinatorConfig(config map[string]interface{}) error {
	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", d.BaseUrl+"/druid/coordinator/v1/config", bytes.NewBuffer(jsonConfig))
	req.SetBasicAuth(d.UserName, d.PassWord)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	fmt.Println(resp.Status)
	return nil
}

func (d DruidClient) GetHistoricalUsage() (map[string]interface{}, error) {
	payload := map[string]string{
		"query": "SELECT \"server\" AS \"service\", \"tier\", \"curr_size\", \"max_size\" FROM sys.servers WHERE server_type = 'historical' ORDER BY \"service\" DESC",
	}
	jsonPayload, err := json.Marshal(payload)
	req, err := http.NewRequest("POST", d.BaseUrl+"/druid/v2/sql", bytes.NewBuffer(jsonPayload))
	req.SetBasicAuth(d.UserName, d.PassWord)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)

	bodyText, err := io.ReadAll(resp.Body)
	s := string(bodyText)
	var m []map[string]interface{}
	usageMap := make(map[string]interface{})
	json.Unmarshal([]byte(s), &m)
	if err != nil {
		return nil, err
	}
	for _, stat := range m {
		usageMap[strings.Split(stat["service"].(string), ".")[0]] = stat["curr_size"]
	}
	return usageMap, nil
}
