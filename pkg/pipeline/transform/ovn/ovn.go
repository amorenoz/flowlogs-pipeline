package ovn

import (
	"context"
	"fmt"
	"strings"

	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/model"
	log "github.com/sirupsen/logrus"
)

type (
	LogicalFlowPipeline = string
)

var (
	LogicalFlowPipelineIngress LogicalFlowPipeline = "ingress"
	LogicalFlowPipelineEgress  LogicalFlowPipeline = "egress"

	Data OVNData
)

// LogicalFlow defines an object in Logical_Flow table
type LogicalFlow struct {
	UUID    string `ovsdb:"_uuid"`
	Actions string `ovsdb:"actions"`
	//	ControllerMeter *string             `ovsdb:"controller_meter"`
	ExternalIDs     map[string]string   `ovsdb:"external_ids"`
	LogicalDatapath *string             `ovsdb:"logical_datapath"`
	LogicalDpGroup  *string             `ovsdb:"logical_dp_group"`
	Match           string              `ovsdb:"match"`
	Pipeline        LogicalFlowPipeline `ovsdb:"pipeline"`
	Priority        int                 `ovsdb:"priority"`
	TableID         int                 `ovsdb:"table_id"`
	Tags            map[string]string   `ovsdb:"tags"`
}

type SampleInfo struct {
	LogicalFlow LogicalFlow
}

type OVNData struct {
	client client.Client
}

func (o *OVNData) GetSampleInfo(observationPointId float64) (*SampleInfo, error) {
	lf := []LogicalFlow{}
	obsString := fmt.Sprintf("%x", int(observationPointId))

	err := o.client.WhereCache(
		func(ls *LogicalFlow) bool {
			return strings.HasPrefix(ls.UUID, obsString)
		}).List(context.Background(), &lf)
	if err != nil {
		return nil, err
	}
	if len(lf) == 0 {
		return nil, fmt.Errorf("No LogicalFlow found with observationPointId %x", obsString)
	}
	if len(lf) > 1 {
		log.Warningf("Duplicated LogicalFlow found with observationPointId %x", obsString)
	}
	return &SampleInfo{
		LogicalFlow: lf[0],
	}, nil
}

func (o *OVNData) InitFromConfig(connection string) error {
	if connection == "" {
		connection = "unix:/var/run/ovn/ovnsb_db.sock"
	}
	log.Infof("Initializing OVN SB connection against %s", connection)
	var err error

	dbModel, err := model.NewClientDBModel("OVN_Southbound",
		map[string]model.Model{
			"Logical_Flow": &LogicalFlow{},
		})
	if err != nil {
		return err
	}

	o.client, err = client.NewOVSDBClient(dbModel, client.WithEndpoint(connection))
	if err != nil {
		return err
	}
	err = o.client.Connect(context.Background())
	if err != nil {
		return err
	}
	_, err = o.client.Monitor(context.TODO(), o.client.NewMonitor(
		client.WithTable(&LogicalFlow{})))
	if err != nil {
		return err
	}

	return nil
}
