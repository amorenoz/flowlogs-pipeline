
[![pull request](https://github.com/netobserv/flowlogs-pipeline/actions/workflows/pull_request.yml/badge.svg)](https://github.com/netobserv/flowlogs-pipeline/actions/workflows/pull_request.yml)
[![push image to quay.io](https://github.com/netobserv/flowlogs-pipeline/actions/workflows/push_image.yml/badge.svg)](https://github.com/netobserv/flowlogs-pipeline/actions/workflows/push_image.yml)
[![codecov](https://codecov.io/gh/netobserv/flowlogs-pipeline/branch/main/graph/badge.svg?token=KMZKG6PRS9)](https://codecov.io/gh/netobserv/flowlogs-pipeline)

# Overview

**Flow-Logs Pipeline** (a.k.a. FLP) is an **observability tool** that consumes raw **network flow-logs** in
their original format 
([NetFlow v5,v9](https://en.wikipedia.org/wiki/NetFlow) or [IPFIX](https://en.wikipedia.org/wiki/IP_Flow_Information_Export)) 
and uses a pipe-line to transform the logs into 
time series metrics in **[prometheus](https://prometheus.io/)** format and in parallel
transform and persist the logs also into **[loki](https://grafana.com/oss/loki/)**.

![Animated gif](docs/images/animation.gif)

FLP decorates the metrics and the transformed logs with **context**, 
allowing visualization layers and analytics frameworks to present **network insights** to SRE’s, cloud operators and network experts.

It also allows defining mathematical transformations to generate condense metrics that encapsulate network domain knowledge.

FLP pipe-line module is built on top of [gopipes](https://github.com/netobserv/gopipes) providing customizability and parallelism

In addition, along with Prometheus and its ecosystem tools such as Thanos, Cortex etc., 
FLP provides an efficient scalable multi-cloud solution for comprehensive network analytics that can rely **solely on metrics data-source**.

Default metrics are documented here [docs/metrics.md](docs/metrics.md).
<br>
<br>

> Note: prometheus eco-system tools such as Alert Manager can be used with FLP to generate alerts and provide big-picture insights.


![Data flow](docs/images/data_flow.drawio.png "Data flow")

# Usage

<!---AUTO-flowlogs-pipeline_help--->
```bash
Expose network flow-logs from metrics  
  
Usage:  
  flowlogs-pipeline [flags]  
  
Flags:  
      --config string        config file (default is $HOME/.flowlogs-pipeline)  
      --health.port string   Health server port (default "8080")  
  -h, --help                 help for flowlogs-pipeline  
      --log-level string     Log level: debug, info, warning, error (default "error")  
      --parameters string    json of config file parameters field  
      --pipeline string      json of config file pipeline field
```
<!---END-AUTO-flowlogs-pipeline_help--->

> Note: for API details refer to  [docs/api.md](docs/api.md).
> 
## Configuration generation

flowlogs-pipeline network metrics configuration ( `--config` flag) can be generated automatically using 
the `confGenerator` utility. `confGenerator` aggregates information from multiple user provided network *metric 
definitions* into flowlogs-pipeline configuration. More details on `confGenerator` can be found 
in [docs/confGenrator.md](docs/confGenerator.md).
 
To generate flowlogs-pipeline configuration execute:  
```shell
make generate-configuration
make dashboards
```

## Deploy into OpenShift (OCP) with prometheus, loki and grafana
To deploy FLP on OCP perform the following steps:
1. Verify that `kubectl` works with the OCP cluster
```shell
kubectl get namespace openshift
```
2. Deploy FLP with all dependent components (into `default` namespace)
```shell
kubectl config set-context --current --namespace=default
make ocp-deploy
```

3. Use a web-browser to access grafana dashboards ( end-point address exposed by the script) and observe metrics and logs  

## Deploy with Kind and netflow-simulator (for development and exploration)
These instructions apply for deploying FLP development and exploration environment with [kind](https://kind.sigs.k8s.io/) and [netflow-simulator](https://hub.docker.com/r/networkstatic/nflow-generator),
tested on Ubuntu 20.4 and Fedora 34.
1. Make sure the following commands are installed and can be run from the current shell:
   - make
   - go (version 1.17)
   - docker
2. To deploy the full simulated environment which includes a kind cluster with FLP, Prometheus, Grafana, and
   netflow-simulator, run (note that depending on your user permissions, you may have to run this command under sudo):
    ```shell
    make local-deploy
    ````
   If the command is successful, the metrics will get generated and can be observed by running (note that depending
   on your user permissions, you may have to run this command under sudo):
    ```shell
    kubectl logs -l app=flowlogs-pipeline -f
    ```
    The metrics you see upon deployment are default and can be modified through configuration described [later](#Configuration).

# Technology

FLP is a framework. The main FLP object is the **pipeline**. FLP **pipeline** can be configured (see
[Configuration section](#Configuration)) to extract the flow-log records from a source in a standard format such as NetFLow or IPFIX, apply custom processing, and output the result as metrics (e.g., in Prometheus format).

# Architecture

The pipeline is constructed of a sequence of stages. Each stage is classified into one of the following types:
- **ingest** - obtain flows from some source, one entry per line
- **decode** - parse input lines into a known format, e.g., dictionary (map) of AWS or goflow data
- **transform** - convert entries into a standard format; can include multiple transform stages
- **write** - provide the means to write the data to some target, e.g. loki, standard output, etc
- **extract** - derive a set of metrics from the imported flows
- **encode** - make the data available in appropriate format (e.g. prometheus)

The first stage in a pipeline must be an **ingest** stage.
The **ingest** stage is typically followed by a **decode** stage, unless the **ingest** stage also performs the decoding.
Each stage (other than an **ingest** stage) specifies the stage it follows.
Multiple stages may follow from a particular stage, thus allowing the same data to be consumed by multiple parallel pipelines.
For example, multiple **transform** stages may be performed and the results may be output to different targets.

A configuration file consists of two sections.
The first section describes the high-level flow of information between the stages, giving each stage a name and building the graph of consumption and production of information between stages.
The second section provides the definition of specific configuration parameters for each one of the named stages.
A full configuration file with the data consumed by  two different transforms might look like the following.

```yaml
pipeline:
  - name: ingest1
  - name: decode1
    follows: ingest1
  - name: generic1
    follows: decode1
  - name: write1
    follows: generic1
  - name: generic2
    follows: decode1
  - name: write2
    follows: generic2
parameters:
  - name: ingest1
    ingest:
      type: file_loop
      file:
        filename: hack/examples/ocp-ipfix-flowlogs.json
  - name: decode1
    decode:
      type: json
  - name: generic1
    transform:
      type: generic
      generic:
        rules:
          - input: Bytes
            output: v1_bytes
          - input: DstAddr
            output: v1_dstAddr
          - input: Packets
            output: v1_packets
          - input: SrcPort
            output: v1_srcPort
  - name: write1
    write:
      type: stdout
  - name: generic2
    transform:
      type: generic
      generic:
        rules:
          - input: Bytes
            output: v2_bytes
          - input: DstAddr
            output: v2_dstAddr
          - input: Packets
            output: v2_packets
          - input: SrcPort
            output: v2_srcPort
  - name: write2
    write:
      type: stdout
```
It is expected that the **ingest** module will receive flows every so often, and this ingestion event will then trigger the rest of the pipeline. So, it is the responsibility of the **ingest** module to provide the timing of when (and how often) the pipeline will run.

# Configuration

It is possible to configure flowlogs-pipeline using command-line-parameters, configuration file, or any combination of those options.


For example:
1. Using configuration file:

```yaml
log-level: info
pipeline:
  - name: ingest_file
  - name: decode_json
    follows: ingest_file
  - name: write_stdout
    follows: write_stdout
parameters
  - name ingest_file
    ingest:
      type: file
      file:
        filename: hack/examples/ocp-ipfix-flowlogs.json
  - name: decode_json
    decode:
      type: json
  - name: write_stdout
    write:
      type: stdout
```
- execute


`./flowlogs-pipeline --config <configFile>`

2. Using command line parameters:
 
`./flowlogs-pipeline --pipeline "[{\"name\":\"ingest1\"},{\"follows\":\"ingest1\",\"name\":\"decode1\"},{\"follows\":\"decode1\",\"name\":\"write1\"}]" --parameters "[{\"ingest\":{\"file\":{\"filename\":\"hack/examples/ocp-ipfix-flowlogs.json\"},\"type\":\"file\"},\"name\":\"ingest1\"},{\"decode\":{\"type\":\"json\"},\"name\":\"decode1\"},{\"name\":\"write1\",\"write\":{\"type\":\"stdout\"}}]"`

Options included in the command line override the options specified in the config file.

`flowlogs-pipeline --log-level debug --pipeline "[{\"name\":\"ingest1\"},{\"follows\":\"ingest1\",\"name\":\"decode1\"},{\"follows\":\"decode1\",\"name\":\"write1\"}]" --config <configFile>`

3. TODO: environment variables

Supported options are provided by running:

```
flowlogs-pipeline --help
```

# Syntax of portions of the configuration file

## Supported stage types

### Transform
Different types of inputs come with different sets of keys.
The transform stage allows changing the names of the keys and deriving new keys from old ones.
Multiple transforms may be specified, and they are applied in the **order of specification** (using the **follows** keyword).
The output from one transform becomes the input to the next transform.

### Transform Generic

The generic transform module maps the input json keys into another set of keys.
This allows to perform subsequent operations using a uniform set of keys.
In some use cases, only a subset of the provided fields are required.
Using the generic transform, we may specify those particular fields that interest us.

For example, suppose we have a flow log with the following syntax:
```
{"Bytes":20800,"DstAddr":"10.130.2.2","DstPort":36936,"Packets":400,"Proto":6,"SequenceNum":1919,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcPort":3100,"TCPFlags":0,"TimeFlowStart":0,"TimeReceived":1637501832}
```

Suppose further that we are only interested in fields with source/destination addresses and ports, together with bytes and packets transferred.
The yaml specification for these parameters would look like this:

```yaml
parameters:
  - name: transform1
    transform:
      type: generic
      generic:
        rules:
          - input: Bytes
            output: bytes
          - input: DstAddr
            output: dstAddr
          - input: DstPort
            output: dstPort
          - input: Packets
            output: packets
          - input: SrcAddr
            output: srcAddr
          - input: SrcPort
            output: srcPort
          - input: TimeReceived
            output: timestamp
```

Each field specified by `input` is translated into a field specified by the corresponding `output`.
Only those specified fields are saved for further processing in the pipeline.
Further stages in the pipeline should use these new field names.
This mechanism allows us to translate from any flow-log layout to a standard set of field names.
If the `input` and `output` fields are identical, then that field is simply passed to the next stage.
For example:
```yaml
pipeline:
  - name: transform1
    follows: <something>
  - name: transform2
    follows: transform1
parameters:
  - name: transform1
    transform
      type: generic
      generic:
        rules:
          - input: DstAddr
            output: dstAddr
          - input: SrcAddr
            output: srcAddr
  - name: transform2
    transform
      type: generic
      generic:
        rules:
          - input: dstAddr
            output: dstIP
          - input: dstAddr
            output: dstAddr
          - input: srcAddr
            output: srcIP
          - input: srcAddr
            output: srcAddr
```
Before the first transform suppose we have the keys `DstAddr` and `SrcAddr`.
After the first transform, we have the keys `dstAddr` and `srcAddr`.
After the second transform, we have the keys `dstAddr`, `dstIP`, `srcAddr`, and `srcIP`.

### Transform Filter

The filter transform module allows setting rules to remove complete entries from
the output, or just remove specific keys and values from entries.

For example, suppose we have a flow log with the following syntax:
```
{"Bytes":20800,"DstAddr":"10.130.2.2","DstPort":36936,"Packets":400,"Proto":6,"SequenceNum":1919,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcPort":3100,"TCPFlags":0,"TimeFlowStart":0,"TimeReceived":1637501832}
```

The below configuration will remove (filter) the entry from the output

```yaml
pipeline:
  transform:
    - type: filter
      filter:
        rules:
        - input: SrcPort
          type: remove_entry_if_exists 
```
Using `remove_entry_if_doesnt_exist` in the rule reverses the logic and will not remove the above example entry
Using `remove_field` in the rule `type` instead, results in outputting the entry after
removal of only the `SrcPort` key and value 

### Transform Network

`transform network` provides specific functionality that is useful for transformation of network flow-logs:

1. Resolve subnet from IP addresses
1. Resolve known network service names from port numbers and protocols
1. Perform simple mathematical transformations on field values
1. Compute geo-location from IP addresses
1. Resolve kubernetes information from IP addresses
1. Perform regex operations on field values

Example configuration:

```yaml
parameters:
  - name: transform1
    transform:
      type: network
      network:
        KubeConfigPath: /tmp/config
        rules:
          - input: srcIP
            output: srcSubnet
            type: add_subnet
            parameters: /24
          - input: value
            output: value_smaller_than10
            type: add_if
            parameters: <10
          - input: dstPort
            output: service
            type: add_service
            parameters: protocol
          - input: dstIP
            output: dstLocation
            type: add_location
          - input: srcIP
            output: srcK8S
            type: add_kubernetes
          - input: srcSubnet
            output: match-10.0
            type: add_regex_if
            parameters: 10.0.*
          - input: "{{.srcIP}},{{.srcPort}},{{.dstIP}},{{.dstPort}},{{.protocol}}"
            output: isNewFlow
            type: conn_tracking
            parameters: "1"
```

The first rule `add_subnet` generates a new field named `srcSubnet` with the 
subnet of `srcIP` calculated based on prefix length from the `parameters` field 

The second `add_if` generates a new field named `value_smaller_than10` that contains 
the contents of the `value` field for entries that satisfy the condition specified 
in the `parameters` variable (smaller than 10 in the example above). In addition, the
field `value_smaller_than10_Evaluate` with value `true` is added to all satisfied
entries

The third rule `add_service` generates a new field named `service` with the known network 
service name of `dstPort` port and `protocol` protocol. Unrecognized ports are ignored 
> Note: `protocol` can be either network protocol name or number

The fourth rule `add_location` generates new fields with the geo-location information retrieved 
from DB [ip2location](https://lite.ip2location.com/) based on `dstIP` IP. 
All the geo-location fields will be named by appending `output` value 
(`dstLocation` in the example above) to their names in the [ip2location](https://lite.ip2location.com/ DB 
(e.g., `CountryName`, `CountryLongName`, `RegionName`, `CityName` , `Longitude` and `Latitude`)

The fifth rule `add_kubernetes` generates new fields with kubernetes information by
matching the `input` value (`srcIP` in the example above) with kubernetes `nodes`, `pods` and `services` IPs.
All the kubernetes fields will be named by appending `output` value
(`srcK8S` in the example above) to the kubernetes metadata field names
(e.g., `Namespace`, `Name`, `Type`, `HostIP`, `OwnerName`, `OwnerType` )

In addition, if the `parameters` value is not empty, fields with kubernetes labels 
will be generated, and named by appending `parameters` value to the label keys.   

> Note: kubernetes connection is done using the first available method: 
> 1. configuration parameter `KubeConfigPath` (in the example above `/tmp/config`) or
> 2. using `KUBECONFIG` environment variable
> 3. using local `~/.kube/config`

The sixth rule `add_regex_if` generates a new field named `match-10.0` that contains
the contents of the `srcSubnet` field for entries that match regex expression specified 
in the `parameters` variable. In addition, the field `match-10.0_Matched` with 
value `true` is added to all matched entries

The seventh rule `conn_tracking` generates a new field named `isNewFlow` that contains
the contents of the `parameters` variable **only for new entries** (first seen in 120 seconds) 
that match hash of template fields from the `input` variable. 


> Note: above example describes all available transform network `Type` options

> Note: above transform is essential for the `aggregation` phase  

### Aggregates

Aggregates are used to define the transformation of flow-logs from textual/json format into
numeric values to be exported as metrics. Aggregates are dynamically created based
on defined values from fields in the flow-logs and on mathematical functions to be performed
on these values.
The specification of the aggregates details is placed in the `extract` stage of the pipeline.

For Example, assuming set of flow-logs, with single sample flow-log that looks like:
```
{"srcIP":   "10.0.0.1",
"dstIP":   "20.0.0.2",
"level":   "error",
"value":   "7",
"message": "test message"}
```

It is possible to define aggregates per `srcIP` or per `dstIP` of per the tuple `srcIP`x`dstIP`
to capture the `sum`, `min`, `avg` etc. of the values in the field `value`.

For example, configuration record for aggregating field `value` as
average for `srcIP`x`dstIP` tuples will look like this::

```yaml
pipeline:
  - name: aggregate1
    follows: <something>
parameters:
  - name: aggregate1
    extract:
      type: aggregates
      aggregates:
        - Name: "Average key=value for (srcIP, dstIP) pairs"
          By:
            - "dstIP"
            - "srcIP"
          Operation: "avg"
          RecordKey: "value"
```

### Prometheus encoder

The prometheus encoder specifies which metrics to export to prometheus and which labels should be associated with those metrics.
For example, we may want to report the number of bytes and packets for the reported flows.
For each reported metric, we may specify a different set of labels.
Each metric may be renamed from its internal name.
The internal metric name is specified as `valuekey` and the exported name is specified as `name`.
A prefix for all exported metrics may be specified, and this prefix is prepended to the `name` of each specified metric.

```yaml
parameters:
  - name: prom1
    encode:
      type: prom
      prom:
        port: 9103
        prefix: test_
        metrics:
          - name: Bytes
            type: gauge
            valuekey: bytes
            labels:
              - srcAddr
              - dstAddr
              - srcPort
          - name: Packets
            type: counter
            valuekey: packets
            labels:
              - srcAddr
              - dstAddr
              - dstPort
```

In this example, for the `bytes` metric we report with the labels which specify srcAddr, dstAddr and srcPort.
Each different combination of label-values is a distinct gauge reported to prometheus.
The name of the prometheus gauge is set to `test_Bytes` by concatenating the prefix with the metric name.
The `packets` metric is very similar. It makes use of the `counter` prometheus type which adds reported values
to a prometheus counter.

### Loki writer

The loki writer persists flow-logs into [Loki](https://github.com/grafana/loki). The flow-logs are sent with defined 
tenant ID and with a set of static labels and dynamic labels from the record fields. 
For example, sending flow-logs into tenant `theTenant` with labels 
from `foo` and `bar` fields 
and including static label with key `job` with value `flowlogs-pipeline`. 
Additional parameters such as `url` and `batchWait` are defined in 
Loki writer API [docs/api.md](docs/api.md)

```bash
parameters:
  - name: write_loki
    write:
      type: loki
      loki:
        tenantID: theTenant
        loki:
          url: http://loki.default.svc.cluster.local:3100
        staticLabels:
          job: flowlogs-pipeline
        batchWait: 1m
        labels:
          - foo
          - bar
  ```

> Note: to view loki flow-logs in `grafana`: Use the `Explore` tab and choose the `loki` datasource. In the `Log Browser` enter `{job="flowlogs-pipeline"}` and press `Run query` 

# Development

## Build

- Clone this repository from github into a local machine (Linux/X86):
  `git clone git@github.com:netobserv/flowlogs-pipeline.git`
- Change directory into flowlogs-pipeline into:
  `cd flowlogs-pipeline`
- Build the code:
  `make build`

FLP uses `Makefile` to build, tests and deploy. Following is the output of `make help` :

<!---AUTO-makefile_help--->
```bash
  
Usage:  
  make <target>  
  
General  
  help                  Display this help.  
  
Develop  
  lint                  Lint the code  
  build                 Build flowlogs-pipeline executable and update the docs  
  dashboards            Build grafana dashboards  
  docs                  Update flowlogs-pipeline documentation  
  clean                 Clean  
  test                  Test  
  benchmarks            Benchmark  
  run                   Run  
  
Docker  
  push-image            Push latest image  
  
kubernetes  
  deploy                Deploy the image  
  undeploy              Undeploy the image  
  deploy-loki           Deploy loki  
  undeploy-loki         Undeploy loki  
  deploy-prometheus     Deploy prometheus  
  undeploy-prometheus   Undeploy prometheus  
  deploy-grafana        Deploy grafana  
  undeploy-grafana      Undeploy grafana  
  deploy-netflow-simulator  Deploy netflow simulator  
  undeploy-netflow-simulator  Undeploy netflow simulator  
  
kind  
  create-kind-cluster   Create cluster  
  delete-kind-cluster   Delete cluster  
  
metrics  
  generate-configuration  Generate metrics configuration  
  
End2End  
  local-deploy          Deploy locally on kind (with simulated flowlogs)  
  local-cleanup         Undeploy from local kind  
  local-redeploy        Redeploy locally (on current kind)  
  ocp-deploy            Deploy to OCP  
  ocp-cleanup           Undeploy from OCP  
  dev-local-deploy      Deploy locally with simulated netflows
```
<!---END-AUTO-makefile_help--->
