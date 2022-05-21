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

// AuthorizationSpec defines the desired state of Authorization
type AuthorizationSpec struct {
	// Organization is the parent organization within which owns this authorization
	// within the target InfluxData instance.
	Organization string `json:"organization"`
	// Description is a string which describes any useful details
	// regarding the purpose or identity of the authorization token.
	Description string `json:"description"`
	// Permissions is the set of permissions (policy) associated
	// with the authorization token.
	Permissions []Permission `json:"permissions"`

	// Token is a target in which to store the resulting token string
	Token Token `json:"token"`
}

// Permission represents the ability to perform and action
// on a target resource specifier.
type Permission struct {
	Action   Action   `json:"action"`
	Resource Resource `json:"resource"`
}

//+kubebuilder:validation:Enum=read;write

// Action specifies the verb which can be performed on a particular
// target resource specifier.
type Action string

// Resource represents a single or collection of resources of a single type.
type Resource struct {
	ResourceType ResourceType `json:"type"`
	Name         string       `json:"name"`
}

// ResourceType represents the type of a target resource.
type ResourceType string

// Token is a structure which identifies a destination for
// the resulting secret token string generated when creating the
// Authorization in a target instance.
type Token struct {
	SecretSpec *SecretSpec `json:"secretSpec,omitempty"`
}

// SecretSpec defines a specification for defining a Secret.
type SecretSpec struct {
	Namespace string `json:"namespace"`
	// NameTemplate is a template which is supplied with details of the target
	// instance associated with the token being stored.
	NameTemplate string `json:"nameTemplate"`
	// Key is the resulting key in the Secret data field under which the token
	// will be stored.
	Key string `json:"key"`
}

// AuthorizationStatus defines the observed state of Authorization
type AuthorizationStatus struct {
	Instances Instances `json:"instances"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Authorization is the Schema for the authorizations API
type Authorization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorizationSpec   `json:"spec,omitempty"`
	Status AuthorizationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AuthorizationList contains a list of Authorization
type AuthorizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Authorization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Authorization{}, &AuthorizationList{})
}
