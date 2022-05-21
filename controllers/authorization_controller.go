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
	"fmt"

	influxdb "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paradoxv1alpha1 "macro.re/paradox/api/v1alpha1"
)

// AuthorizationReconciler reconciles a Authorization object
type AuthorizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=paradox.macro.re,resources=authorizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=paradox.macro.re,resources=authorizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=paradox.macro.re,resources=authorizations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Authorization object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *AuthorizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var authorization paradoxv1alpha1.Authorization
	if err := r.Get(ctx, req.NamespacedName, &authorization); err != nil {
		log.Error(err, "unable to fetch authorization")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("authorization", authorization)

	var organization paradoxv1alpha1.Organization
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.NamespacedName.Namespace,
		Name:      authorization.Spec.Organization,
	}, &organization); err != nil {
		log.Error(err, "unable to fetch organization")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	status := paradoxv1alpha1.AuthorizationStatus{
		Instances: paradoxv1alpha1.Instances{},
	}

	if err := forEachInstanceClient(ctx, r.Client, &organization, func(instance *paradoxv1alpha1.Instance, iclient influxdb.Client) error {
		namespace, name := instance.ObjectMeta.Namespace, instance.ObjectMeta.Name
		wrapErr := func(err error) error {
			return fmt.Errorf("influx instance '%s/%s': %w", namespace, name, err)
		}

		orgInstance, ok := organization.Status.Instances[namespace][name]
		if !ok || orgInstance.ID == nil {
			return wrapErr(fmt.Errorf("organization does not have an ID"))
		}

		var (
			authAPI = iclient.AuthorizationsAPI()
			auth    *domain.Authorization
		)

		authInstance, ok := authorization.Status.Instances[namespace][name]
		if !ok || authInstance.ID == nil {
			permissions := []domain.Permission{}
			auth = &domain.Authorization{
				AuthorizationUpdateRequest: domain.AuthorizationUpdateRequest{
					Description: &authorization.Spec.Description,
				},
				OrgID:       toStringPtr(orgInstance.ID),
				Permissions: &permissions,
			}

			for _, permission := range authorization.Spec.Permissions {
				perm := domain.Permission{
					Action: domain.PermissionAction(permission.Action),
					Resource: domain.Resource{
						Type:  domain.ResourceType(permission.Resource.ResourceType),
						OrgID: toStringPtr(orgInstance.ID),
					},
				}

				switch perm.Resource.Type {
				case "buckets":
					var bucket paradoxv1alpha1.Bucket
					if err := r.Get(ctx, types.NamespacedName{
						Namespace: req.NamespacedName.Namespace,
						Name:      permission.Resource.Name,
					}, &bucket); err != nil {
						return err
					}

					bucketInstance := bucket.Status.Instances[namespace][name]

					perm.Resource.Id = toStringPtr(bucketInstance.ID)
				default:
					return wrapErr(fmt.Errorf("unsupported resource type %q", perm.Resource.Type))
				}

				*auth.Permissions = append(*auth.Permissions, perm)
			}

			var err error
			auth, err = authAPI.CreateAuthorization(ctx, auth)
			if err != nil {
				return wrapErr(err)
			}

			status.Instances.AddInstance(
				instance,
				fromStringPtr[paradoxv1alpha1.InfluxID](auth.Id),
			)

			return nil
		}

		status.Instances.AddInstance(
			instance,
			authInstance.ID,
		)

		return nil
	}); err != nil {
		log.Error(err, "error while configuring instances")

		return ctrl.Result{}, err
	}

	if status.Instances != nil {
		authorization.Status = status

		if err := r.Status().Update(ctx, &authorization); err != nil {
			log.Error(err, "failed to update status")

			return ctrl.Result{}, err
		}

		log.V(4).Info("status updated")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthorizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paradoxv1alpha1.Authorization{}).
		Complete(r)
}
