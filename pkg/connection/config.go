package connection

import (
	"encoding/json"
	"io/ioutil"
)

type dbConfig struct {
	Addr     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

func getDbConfig(filename string) (*dbConfig, error) {
	config := dbConfig{}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
