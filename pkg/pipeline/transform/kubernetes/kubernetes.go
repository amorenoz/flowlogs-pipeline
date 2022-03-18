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

package kubernetes

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var Data KubeData

const (
	kubeConfigEnvVariable = "KUBECONFIG"
	syncTime              = 10 * time.Minute
	IndexIP               = "byIP"
	//IndexIngressOPID      = "byIngressObservationPointID"
	//IndexEgressOPID       = "byEgressObservationPointID"
	typeNode          = "Node"
	typePod           = "Pod"
	typeService       = "Service"
	typeNetworkPolicy = "NetworkPolicy"
	typeNamespace     = "Namespace"
)

type KubeData struct {
	ipInformers        map[string]cache.SharedIndexInformer
	sampleInformers    map[ACLSampleType]cache.SharedIndexInformer
	replicaSetInformer cache.SharedIndexInformer
	stopChan           chan struct{}
}

type Owner struct {
	Type string
	Name string
}

type IPInfo struct {
	Type            string
	Name            string
	Namespace       string
	Labels          map[string]string
	OwnerReferences []metav1.OwnerReference
	Owner           Owner
	HostIP          string
}

type ACLSampleType string

const (
	ACLSampleTypeNetworkPolicy ACLSampleType = "networkPolicy"
	ACLSampleTypeNamespace     ACLSampleType = "namespace"
)

type ACLSampleDirection = string

const (
	ACLSampleDirectionIngress ACLSampleDirection = "ingress"
	ACLSampleDirectionEgress  ACLSampleDirection = "egress"
)

const aclSamplingAnnotation = "k8s.ovn.org/acl-sampling"

type GressACLSampling struct {
	ObservationPointID int `json:"observation_point_id,omitempty"`
	Probability        int `json:"probability,omitempty"`
}

type AclSampling struct {
	Ingress *GressACLSampling `json:"ingress,omitempty"`
	Egress  *GressACLSampling `json:"egress,omitempty"`
}

type ACLSampleInfo struct {
	Type          ACLSampleType
	Direction     ACLSampleDirection
	Namespace     string
	NetworkPolicy string
}

func (k *KubeData) GetSampleInfo(obsPointID int) (*ACLSampleInfo, error) {
	obsPointIDstr := strconv.FormatUint(uint64(obsPointID), 10)
	for objType, informer := range k.sampleInformers {
		for _, direction := range []ACLSampleDirection{ACLSampleDirectionIngress, ACLSampleDirectionEgress} {
			obs, err := informer.GetIndexer().ByIndex(direction, obsPointIDstr)
			if err == nil && len(obs) > 0 {
				var info *ACLSampleInfo
				switch objType {
				case typeNamespace:
					namespace := obs[0].(*v1.Namespace)
					info = &ACLSampleInfo{
						Type:      ACLSampleTypeNamespace,
						Direction: direction,
						Namespace: namespace.Name,
					}
				case typeNetworkPolicy:
					np := obs[0].(*netv1.NetworkPolicy)
					info = &ACLSampleInfo{
						Type:          ACLSampleTypeNetworkPolicy,
						Direction:     direction,
						NetworkPolicy: np.Name,
					}
				}
				return info, nil
			}
		}
	}
	return nil, fmt.Errorf("Cannot find ObservationPointID %d", obsPointID)
}

func (k *KubeData) GetIPInfo(ip string) (*IPInfo, error) {
	for objType, informer := range k.ipInformers {
		objs, err := informer.GetIndexer().ByIndex(IndexIP, ip)
		if err == nil && len(objs) > 0 {
			var info *IPInfo
			switch objType {
			case typePod:
				pod := objs[0].(*v1.Pod)
				info = &IPInfo{
					Type:            typePod,
					Name:            pod.Name,
					Namespace:       pod.Namespace,
					Labels:          pod.Labels,
					OwnerReferences: pod.OwnerReferences,
					HostIP:          pod.Status.HostIP,
				}
			case typeNode:
				node := objs[0].(*v1.Node)
				info = &IPInfo{
					Type:      typeNode,
					Name:      node.Name,
					Namespace: node.Namespace,
					Labels:    node.Labels,
				}
			case typeService:
				service := objs[0].(*v1.Service)
				info = &IPInfo{
					Type:      typeService,
					Name:      service.Name,
					Namespace: service.Namespace,
					Labels:    service.Labels,
				}
			}

			info.Owner = k.getOwner(info)
			return info, nil
		}
	}

	return nil, fmt.Errorf("can't find ip")
}

func (k *KubeData) getOwner(info *IPInfo) Owner {
	if info.OwnerReferences != nil && len(info.OwnerReferences) > 0 {
		ownerReference := info.OwnerReferences[0]
		if ownerReference.Kind == "ReplicaSet" {
			item, ok, err := k.replicaSetInformer.GetIndexer().GetByKey(info.Namespace + "/" + ownerReference.Name)
			if err != nil {
				panic(err)
			}
			if ok {
				replicaSet := item.(*appsv1.ReplicaSet)
				if len(replicaSet.OwnerReferences) > 0 {
					return Owner{
						Name: replicaSet.OwnerReferences[0].Name,
						Type: replicaSet.OwnerReferences[0].Kind,
					}
				}
			}
		} else {
			return Owner{
				Name: ownerReference.Name,
				Type: ownerReference.Kind,
			}
		}
	}

	return Owner{
		Name: info.Name,
		Type: info.Type,
	}
}

func (k *KubeData) NewNodeInformer(informerFactory informers.SharedInformerFactory) error {
	nodes := informerFactory.Core().V1().Nodes().Informer()
	err := nodes.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			node := obj.(*v1.Node)
			ips := make([]string, 0, len(node.Status.Addresses))
			for _, address := range node.Status.Addresses {
				ip := net.ParseIP(address.Address)
				if ip != nil {
					ips = append(ips, ip.String())
				}
			}
			return ips, nil
		},
	})

	k.ipInformers[typeNode] = nodes
	return err
}

func (k *KubeData) NewPodInformer(informerFactory informers.SharedInformerFactory) error {
	pods := informerFactory.Core().V1().Pods().Informer()
	err := pods.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			pod := obj.(*v1.Pod)
			ips := make([]string, 0, len(pod.Status.PodIPs))
			for _, ip := range pod.Status.PodIPs {
				// ignoring host-networked Pod IPs
				if ip.IP != pod.Status.HostIP {
					ips = append(ips, ip.IP)
				}
			}
			return ips, nil
		},
	})

	k.ipInformers[typePod] = pods
	return err
}

func (k *KubeData) NewServiceInformer(informerFactory informers.SharedInformerFactory) error {
	services := informerFactory.Core().V1().Services().Informer()
	err := services.AddIndexers(map[string]cache.IndexFunc{
		IndexIP: func(obj interface{}) ([]string, error) {
			service := obj.(*v1.Service)
			ips := service.Spec.ClusterIPs
			if service.Spec.ClusterIP == v1.ClusterIPNone {
				return []string{}, nil
			}
			return ips, nil
		},
	})

	k.ipInformers[typeService] = services
	return err
}

func getObsPoint(obj metav1.Object) (string, string, error) {
	var ingress, egress string
	samplingAnnotation, ok := obj.GetAnnotations()[aclSamplingAnnotation]
	if !ok {
		return "", "", nil
	}
	config := AclSampling{}
	if err := json.Unmarshal([]byte(samplingAnnotation), &config); err != nil {
		return "", "", fmt.Errorf("Failed parsing acl-sampling annotation %v", err)
	}
	if config.Ingress != nil {
		ingress = strconv.FormatUint(uint64(config.Ingress.ObservationPointID), 10)
	}
	if config.Egress != nil {
		egress = strconv.FormatUint(uint64(config.Egress.ObservationPointID), 10)
	}
	return ingress, egress, nil

}
func (k *KubeData) NewNetworkPolicyInformer(informerFactory informers.SharedInformerFactory) error {
	nps := informerFactory.Networking().V1().NetworkPolicies().Informer()
	err := nps.AddIndexers(map[string]cache.IndexFunc{
		ACLSampleDirectionIngress: func(obj interface{}) ([]string, error) {
			np := obj.(*netv1.NetworkPolicy)
			ingress, _, err := getObsPoint(np)
			if err != nil {
				return []string{}, err
			}
			if ingress != "" {
				return []string{ingress}, nil
			}
			return []string{}, nil
		},
		ACLSampleDirectionEgress: func(obj interface{}) ([]string, error) {
			np := obj.(*netv1.NetworkPolicy)
			_, egress, err := getObsPoint(np)
			if err != nil {
				return []string{}, err
			}
			if egress != "" {
				return []string{egress}, nil
			}
			return []string{}, nil
		},
	})

	k.sampleInformers[typeNetworkPolicy] = nps
	return err
}

func (k *KubeData) NewNamespaceInformer(informerFactory informers.SharedInformerFactory) error {
	namespaces := informerFactory.Core().V1().Namespaces().Informer()
	err := namespaces.AddIndexers(map[string]cache.IndexFunc{
		ACLSampleDirectionIngress: func(obj interface{}) ([]string, error) {
			np := obj.(*v1.Namespace)
			ingress, _, err := getObsPoint(np)
			if err != nil {
				return []string{}, err
			}
			if ingress != "" {
				return []string{ingress}, nil
			}
			return []string{}, nil
		},
		ACLSampleDirectionEgress: func(obj interface{}) ([]string, error) {
			np := obj.(*v1.Namespace)
			_, egress, err := getObsPoint(np)
			if err != nil {
				return []string{}, err
			}
			if egress != "" {
				return []string{egress}, nil
			}
			return []string{}, nil
		},
	})

	k.sampleInformers[typeNamespace] = namespaces
	return err
}

func (k *KubeData) NewReplicaSetInformer(informerFactory informers.SharedInformerFactory) error {
	k.replicaSetInformer = informerFactory.Apps().V1().ReplicaSets().Informer()
	return nil
}

func (k *KubeData) InitFromConfig(kubeConfigPath string) error {
	// Initialization variables
	k.stopChan = make(chan struct{})
	k.ipInformers = map[string]cache.SharedIndexInformer{}
	k.sampleInformers = map[ACLSampleType]cache.SharedIndexInformer{}

	config, err := LoadConfig(kubeConfigPath)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	err = k.initInformers(kubeClient)
	if err != nil {
		return err
	}

	return nil
}

func LoadConfig(kubeConfigPath string) (*rest.Config, error) {
	// if no config path is provided, load it from the env variable
	if kubeConfigPath == "" {
		kubeConfigPath = os.Getenv(kubeConfigEnvVariable)
	}
	// otherwise, load it from the $HOME/.kube/config file
	if kubeConfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("can't get user home dir: %w", err)
		}
		kubeConfigPath = path.Join(homeDir, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err == nil {
		return config, nil
	}
	// fallback: use in-cluster config
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can't access kubenetes. Tried using config from: "+
			"config parameter, %s env, homedir and InClusterConfig. Got: %w",
			kubeConfigEnvVariable, err)
	}
	return config, nil
}

func (k *KubeData) initInformers(client kubernetes.Interface) error {
	informerFactory := informers.NewSharedInformerFactory(client, syncTime)
	err := k.NewNodeInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewPodInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewServiceInformer(informerFactory)
	if err != nil {
		return err
	}
	err = k.NewReplicaSetInformer(informerFactory)
	if err != nil {
		return err
	}

	err = k.NewNetworkPolicyInformer(informerFactory)
	if err != nil {
		return err
	}

	err = k.NewNamespaceInformer(informerFactory)
	if err != nil {
		return err
	}

	log.Debugf("starting kubernetes informers, waiting for syncronization")
	informerFactory.Start(k.stopChan)
	informerFactory.WaitForCacheSync(k.stopChan)
	log.Debugf("kubernetes informers started")

	return nil
}
