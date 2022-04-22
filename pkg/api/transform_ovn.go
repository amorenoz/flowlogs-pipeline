package api

type TransformOVN struct {
	Rules   OVNTransformRules `yaml:"rules" doc:"list of transform rules, each includes:"`
	OVNSBDB string            `yaml:"ovn_sb_db" json:"ovn_sb_db" doc:"string expressing the way to connect to the OVN SB Database (default: unix:/var/run/ovn/ovnsb_db.sock)"`
}

type TransformOVNOperationEnum struct {
	LogicalFlowUUID       string `yaml:"lflow_uuid" doc:"set output field to the UUID of the logical flow that created the sample"`
	LogicalFlowMatch      string `yaml:"lflow_match" doc:"set output field to the match of the logical flow that created the sample"`
	LogicalFlowActions    string `yaml:"lflow_actions" doc:"set output field to the action of the logical flow that created the sample"`
	LogicalFlowPriority   string `yaml:"lflow_prio" doc:"set output field to the of the logical flow that created the sample"`
	LogicalFlowExternalID string `yaml:"lflow_ext" doc:"set output_{KEY} fields to the correspondent values of the external_id map action of the logical flow that created the sample"`
	LogicalFlowPipeline   string `yaml:"lflow_pipeline" doc:"set output field to the pipeline of the flow that created the sample"`
	LogicalFlowTable      int    `yaml:"lflow_table" doc:"set output field to the table of the flow that created the sample"`
}

func TransformOVNOperationName(operation string) string {
	return GetEnumName(TransformOVNOperationEnum{}, operation)
}

type OVNTransformRule struct {
	Input      string `yaml:"input" doc:"entry input field"`
	Output     string `yaml:"output" doc:"entry output field"`
	Type       string `yaml:"type" enum:"TransformOVNOperationEnum" doc:"one of the following:"`
	Parameters string `yaml:"parameters" doc:"parameters specific to type"`
}

type OVNTransformRules []OVNTransformRule
