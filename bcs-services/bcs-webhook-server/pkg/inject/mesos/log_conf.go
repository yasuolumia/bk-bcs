/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package mesos

import (
	"strconv"

	"bk-bcs/bcs-common/common/blog"
	commtypes "bk-bcs/bcs-common/common/types"
	bcsv2 "bk-bcs/bcs-services/bcs-webhook-server/pkg/apis/bk-bcs/v2"
	listers "bk-bcs/bcs-services/bcs-webhook-server/pkg/client/listers/bk-bcs/v2"
	"bk-bcs/bcs-services/bcs-webhook-server/pkg/inject/common"
	mapset "github.com/deckarep/golang-set"
	"k8s.io/apimachinery/pkg/labels"
)

// LogConfInject implements MesosInject
type LogConfInject struct {
	BcsLogConfigLister listers.BcsLogConfigLister
}

// NewLogConfInject create LogConfInject object
func NewLogConfInject(bcsLogConfLister listers.BcsLogConfigLister) MesosInject {
	mesosInject := &LogConfInject{
		BcsLogConfigLister: bcsLogConfLister,
	}

	return mesosInject
}

// InjectApplicationContent inject log envs to application
func (logConf *LogConfInject) InjectApplicationContent(application *commtypes.ReplicaController) (*commtypes.ReplicaController, error) {
	// get all BcsLogConfig
	bcsLogConfs, err := logConf.BcsLogConfigLister.List(labels.Everything())
	if err != nil {
		blog.Errorf("list bcslogconfig error %s", err.Error())
		return nil, err
	}

	//handle bcs-system modules' log inject
	namespaceSet := mapset.NewSet()
	for _, namespace := range common.IgnoredNamespaces {
		namespaceSet.Add(namespace)
	}
	if namespaceSet.Contains(application.ObjectMeta.NameSpace) {
		matchedLogConf := common.FindBcsSystemConfigType(bcsLogConfs)
		if matchedLogConf != nil {
			injected := logConf.injectMesosContainers(application.ObjectMeta.NameSpace, application.ReplicaControllerSpec.Template, matchedLogConf)
			application.ReplicaControllerSpec.Template = injected
		}
		return application, nil
	}

	// handle business modules log inject
	var injectedContainers []commtypes.Container
	for _, container := range application.ReplicaControllerSpec.Template.PodSpec.Containers {
		matchedLogConf := common.FindMatchedConfigType(container.Name, bcsLogConfs)
		if matchedLogConf != nil {
			injectedContainer := logConf.injectMesosContainer(application.ObjectMeta.NameSpace, container, matchedLogConf)
			injectedContainers = append(injectedContainers, injectedContainer)
		} else {
			injectedContainers = append(injectedContainers, container)
		}
	}
	application.ReplicaControllerSpec.Template.PodSpec.Containers = injectedContainers
	return application, nil
}

// InjectDeployContent inject log envs to Deployment
func (logConf *LogConfInject) InjectDeployContent(deploy *commtypes.BcsDeployment) (*commtypes.BcsDeployment, error) {
	// get all BcsLogConfig
	bcsLogConfs, err := logConf.BcsLogConfigLister.List(labels.Everything())
	if err != nil {
		blog.Errorf("list bcslogconfig error %s", err.Error())
		return nil, err
	}

	//handle bcs-system modules' log inject
	namespaceSet := mapset.NewSet()
	for _, namespace := range common.IgnoredNamespaces {
		namespaceSet.Add(namespace)
	}
	if namespaceSet.Contains(deploy.ObjectMeta.NameSpace) {
		matchedLogConf := common.FindBcsSystemConfigType(bcsLogConfs)
		if matchedLogConf != nil {
			injected := logConf.injectMesosContainers(deploy.ObjectMeta.NameSpace, deploy.Spec.Template, matchedLogConf)
			deploy.Spec.Template = injected
		}
		return deploy, nil
	}

	// handle business modules log inject
	var injectedContainers []commtypes.Container
	for _, container := range deploy.Spec.Template.PodSpec.Containers {
		matchedLogConf := common.FindMatchedConfigType(container.Name, bcsLogConfs)
		if matchedLogConf != nil {
			injectedContainer := logConf.injectMesosContainer(deploy.ObjectMeta.NameSpace, container, matchedLogConf)
			injectedContainers = append(injectedContainers, injectedContainer)
		} else {
			injectedContainers = append(injectedContainers, container)
		}
	}
	deploy.Spec.Template.PodSpec.Containers = injectedContainers
	return deploy, nil
}

// injectMesosContainers injects bcs log config to all containers
func (logConf *LogConfInject) injectMesosContainers(namespace string, podTemplate *commtypes.PodTemplateSpec, bcsLogConf *bcsv2.BcsLogConfig) *commtypes.PodTemplateSpec {

	var injectedContainers []commtypes.Container
	for _, container := range podTemplate.PodSpec.Containers {
		injectedContainer := logConf.injectMesosContainer(namespace, container, bcsLogConf)
		injectedContainers = append(injectedContainers, injectedContainer)
	}

	podTemplate.PodSpec.Containers = injectedContainers
	return podTemplate
}

// injectMesosContainer injects bcs log config to an container
func (logConf *LogConfInject) injectMesosContainer(namespace string, container commtypes.Container, bcsLogConf *bcsv2.BcsLogConfig) commtypes.Container {
	var envs []commtypes.EnvVar
	dataIdEnv := commtypes.EnvVar{
		Name:  common.DataIdEnvKey,
		Value: bcsLogConf.Spec.DataId,
	}
	envs = append(envs, dataIdEnv)

	appIdEnv := commtypes.EnvVar{
		Name:  common.AppIdEnvKey,
		Value: bcsLogConf.Spec.AppId,
	}
	envs = append(envs, appIdEnv)

	stdoutEnv := commtypes.EnvVar{
		Name:  common.StdoutEnvKey,
		Value: strconv.FormatBool(bcsLogConf.Spec.Stdout),
	}
	envs = append(envs, stdoutEnv)

	logPathEnv := commtypes.EnvVar{
		Name:  common.LogPathEnvKey,
		Value: bcsLogConf.Spec.LogPath,
	}
	envs = append(envs, logPathEnv)

	clusterIdEnv := commtypes.EnvVar{
		Name:  common.ClusterIdEnvKey,
		Value: bcsLogConf.Spec.ClusterId,
	}
	envs = append(envs, clusterIdEnv)

	namespaceEnv := commtypes.EnvVar{
		Name:  common.NamespaceEnvKey,
		Value: namespace,
	}
	envs = append(envs, namespaceEnv)

	container.Env = envs

	blog.Infof("%v", container.Env)
	return container
}
