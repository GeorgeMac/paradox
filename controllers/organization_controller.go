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

package controllers

import (
	"context"
	"errors"

	influxdb "github.com/influxdata/influxdb-client-go/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paradoxv1alpha1 "macro.re/paradox/api/v1alpha1"
)

var (
	ErrOrgHasNoAuthorization = errors.New("organization has no associated admin authorization token")

	ErrInfluxUnexpectedResponse = errors.New("target Influx instance returned unexpected response")
)

func toStringPtr[V ~string](v *V) *string {
	if v == nil {
		return nil
	}

	str := string(*v)
	return &str
}

func fromStringPtr[V ~string](str *string) *V {
	if str == nil {
		return nil
	}

	v := V(*str)
	return &v
}

// OrganizationReconciler reconciles a Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=paradox.macro.re,resources=organizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=paradox.macro.re,resources=organizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=paradox.macro.re,resources=organizations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var organization paradoxv1alpha1.Organization
	if err := r.Get(ctx, req.NamespacedName, &organization); err != nil {
		log.Error(err, "unable to fetch organization")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("organization", organization)

	status := paradoxv1alpha1.OrganizationStatus{
		Instances: paradoxv1alpha1.Instances{},
	}

	if err := forEachInstanceClient(ctx, r.Client, &organization, func(instance *paradoxv1alpha1.Instance, client influxdb.Client) error {
		orgAPI := client.OrganizationsAPI()
		org, err := orgAPI.FindOrganizationByName(ctx, organization.Spec.Name)
		if err != nil {
			log.Error(err, "could not fetch from Influx instance")

			// TODO(georgemac): in the future add support for org creation by way of instance
			// provisioning credentials
			return err
		}

		// update target org description if they differ
		if org.Description != nil && *org.Description != organization.Spec.Description {
			org.Description = &organization.Spec.Description
			org, err = orgAPI.UpdateOrganization(ctx, org)
			if err != nil {
				log.Error(err, "could not update target Influx instance")

				return err
			}
		}

		status.Instances.AddInstance(
			instance,
			fromStringPtr[paradoxv1alpha1.InfluxID](org.Id),
		)

		return nil
	}); err != nil {
		log.Error(err, "error while configuring instances")

		return ctrl.Result{}, err
	}

	organization.Status = status

	if err := r.Status().Update(ctx, &organization); err != nil {
		log.Error(err, "failed to update status")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paradoxv1alpha1.Organization{}).
		Complete(r)
}

func forEachInstanceClient(
	ctx context.Context,
	client client.Client,
	organization *paradoxv1alpha1.Organization,
	fn func(instance *paradoxv1alpha1.Instance, client influxdb.Client) error,
) error {
	for namespace, namespacedInstances := range organization.Spec.InstanceRefs {
		for name, auth := range namespacedInstances {
			var instance paradoxv1alpha1.Instance
			if err := client.Get(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &instance); err != nil {
				return err
			}

			// TODO(georgemac): remove hard requirement for token when support is added
			// for secret retrieval
			if auth.Type != paradoxv1alpha1.InstanceAuthorizationTypeToken ||
				auth.Token == nil {
				return ErrOrgHasNoAuthorization
			}

			return fn(&instance, influxdb.NewClient(instance.Spec.Address, *auth.Token))
		}
	}

	return nil
}
