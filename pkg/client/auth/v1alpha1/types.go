// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tenant defines a Tenant deployment.
type Tenant struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the Tenant cluster. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
	Spec TenantSpec `json:"spec"`
}

// TenantList is a list of Tenantes.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Tenantes
	Items []*Tenant `json:"items"`
}

// Specification of the desired behavior of the Tenant cluster. More info:
// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
type TenantSpec struct {
	// UserName name of user
	UserName string `json:"userName,omitempty"`
	//Password pass of user
	Password string `json:"password,omitempty"`
	//TenantID id of Tenant
	TeanantID string `json:"tenantID,omitempty"`
}

//TODO: add network
