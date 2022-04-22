package transform

import (
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/ovn"

	log "github.com/sirupsen/logrus"
)

type OVN struct {
	api.TransformOVN
}

func (n *OVN) Transform(inputEntries []config.GenericMap) []config.GenericMap {
	outputEntries := make([]config.GenericMap, 0)
	for _, entry := range inputEntries {
		outputEntry := n.TransformEntry(entry)
		outputEntries = append(outputEntries, outputEntry)
	}
	return outputEntries
}

func (n *OVN) TransformEntry(inputEntry config.GenericMap) config.GenericMap {
	outputEntries := inputEntry

	obsPointID, ok := outputEntries["ObservationPointID"]
	if !ok {
		log.Errorf("Can't extract OVN information: ObservationPointID field missing in flow entry")
		return outputEntries
	}
	sampleInfo, err := ovn.Data.GetSampleInfo(obsPointID.(float64))
	if err != nil {
		log.Errorf("Error extracting OVN Information from sample: %s", err.Error())
		return outputEntries
	}

	for _, rule := range n.Rules {
		switch rule.Type {
		case api.TransformOVNOperationName("LogicalFlowUUID"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.UUID
		case api.TransformOVNOperationName("LogicalFlowMatch"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.Match
		case api.TransformOVNOperationName("LogicalFlowActions"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.Actions
		case api.TransformOVNOperationName("LogicalFlowPipeline"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.Pipeline
		case api.TransformOVNOperationName("LogicalFlowPriority"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.Priority
		case api.TransformOVNOperationName("LogicalFlowExternalID"):
			for key, value := range sampleInfo.LogicalFlow.ExternalIDs {
				outputEntries[fmt.Sprintf("%s_%s", rule.Output, key)] = value
			}
		case api.TransformOVNOperationName("LogicalFlowTable"):
			outputEntries[rule.Output] = sampleInfo.LogicalFlow.TableID
		default:
			log.Errorf("Unknown OVN operation: %s", rule.Type)
			return outputEntries
		}
	}
	return outputEntries
}

func NewTransformOVN(params config.StageParam) (Transformer, error) {
	jsonOVNTransform := params.Transform.OVN
	err := ovn.Data.InitFromConfig(jsonOVNTransform.OVNSBDB)
	if err != nil {
		return nil, err
	}
	return &OVN{
		api.TransformOVN{
			Rules: jsonOVNTransform.Rules,
		},
	}, nil
}
