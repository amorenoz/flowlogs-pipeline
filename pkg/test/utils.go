/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func GetIngestMockEntry(missingKey bool) config.GenericMap {
	entry := config.GenericMap{
		"srcIP":        "10.0.0.1",
		"8888IP":       "8.8.8.8",
		"emptyIP":      "",
		"level":        "error",
		"srcPort":      11777,
		"protocol":     "tcp",
		"protocol_num": 6,
		"value":        7.0,
		"message":      "test message",
	}

	if !missingKey {
		entry["dstIP"] = "20.0.0.2"
		entry["dstPort"] = 22
	}

	return entry
}

func InitConfig(t *testing.T, conf string) *viper.Viper {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	yamlConfig := []byte(conf)
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader(yamlConfig)
	err := v.ReadConfig(r)
	require.NoError(t, err)

	// set up global config info
	// first clear out the config structures in case they were set by a previous instantiation
	p1 := reflect.ValueOf(&config.PipeLine).Elem()
	p1.Set(reflect.Zero(p1.Type()))
	p2 := reflect.ValueOf(&config.Parameters).Elem()
	p2.Set(reflect.Zero(p2.Type()))

	var b []byte
	pipelineStr := v.Get("pipeline")
	b, err = json.Marshal(&pipelineStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		return nil
	}
	config.Opt.PipeLine = string(b)
	parametersStr := v.Get("parameters")
	b, err = json.Marshal(&parametersStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		return nil
	}
	config.Opt.Parameters = string(b)
	err = json.Unmarshal([]byte(config.Opt.PipeLine), &config.PipeLine)
	if err != nil {
		fmt.Printf("error unmarshaling: %v\n", err)
		return nil
	}
	err = json.Unmarshal([]byte(config.Opt.Parameters), &config.Parameters)
	if err != nil {
		fmt.Printf("error unmarshaling: %v\n", err)
		return nil
	}

	err = config.ParseConfig()
	if err != nil {
		fmt.Printf("error in parsing config file: %v \n", err)
		return nil
	}

	return v
}

func GetExtractMockEntry() config.GenericMap {
	entry := config.GenericMap{
		"srcAddr":         "10.1.2.3",
		"dstAddr":         "10.1.2.4",
		"srcPort":         "9001",
		"dstPort":         "39504",
		"bytes":           "1234",
		"packets":         "34",
		"recentRawValues": []float64{1.1, 2.2},
	}
	return entry
}
