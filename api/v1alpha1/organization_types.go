/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrganizationSpec defines the desired state of Organization
type OrganizationSpec struct {
	// Name is the name as it is defined in the target Influx instances
	Name string `json:"name"`
	// Description is a string which describes any useful details
	// regarding the purpose or identity of the organization.
	Description string `json:"description"`

	// InstanceRefs is a map of namespace -> name -> authorization
	InstanceRefs map[string]map[string]InstanceAuthorization `json:"instance_refs"`
}

type InstanceAuthorization struct {
	Type InstanceAuthorizationType `json:"type"`

	Token  *string    `json:"token,omitempty"`
	Secret *SecretRef `json:"secretRef,omitempty"`
}

type SecretRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Key       string `json:"key"`
}

//+kubebuilder:validation:Enum=token;secret

type InstanceAuthorizationType string

const (
	InstanceAuthorizationTypeToken  = InstanceAuthorizationType("token")
	InstanceAuthorizationTypeSecret = InstanceAuthorizationType("secret")
)

// OrganizationStatus defines the observed state of Organization
type OrganizationStatus struct {
	Instances Instances `json:"instances"`
}

// Instances is a map of namespace to map of name to resource instance.
type Instances map[string]map[string]ResourceInstance

func (i Instances) AddInstance(instance *Instance, id *InfluxID) {
	if id == nil {
		return
	}

	instances, ok := i[instance.ObjectMeta.Namespace]
	if !ok {
		instances = map[string]ResourceInstance{}
		i[instance.ObjectMeta.Namespace] = instances
	}

	instances[instance.ObjectMeta.Name] = ResourceInstance{ID: id}
}

type ResourceInstance struct {
	// ID is the identifier which relates to the named resource
	// in the target InfluxData instance.
	ID *InfluxID `json:"id,omitempty"`
}

// InfluxID is an int64 represented as a hexidecimally encoded string.
type InfluxID string

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=org;orgs

// Organization is the Schema for the organizations API
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec,omitempty"`
	Status OrganizationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
