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

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	// Name is the name of the bucket in the target Influx instance.
	Name string `json:"name"`
	// Organization is the parent organization within which owns this bucket
	// within the target InfluxData instance.
	Organization string `json:"organization"`
	// Description is a string which describes any useful details
	// regarding the purpose or identity of the bucket.
	Description     string     `json:"description,omitempty"`
	SchemaType      SchemaType `json:"schema_type,omitempty"`
	RetentionPolicy string     `json:"retention_policy,omitempty"`
}

//+kubebuilder:validation:default=implicit
//+kubebuilder:validation:Enum=implicit;explicit

type SchemaType string

// BucketStatus defines the observed state of Bucket
type BucketStatus struct {
	Instances Instances `json:"instances"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//*kubebuilder:printcolumn:JSONPath=".spec.organization",name=Organization,type=string

// Bucket is the Schema for the buckets API
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BucketList contains a list of Bucket
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}
