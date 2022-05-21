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
	"strings"
	"time"

	influxdb "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	paradoxv1alpha1 "macro.re/paradox/api/v1alpha1"
)

const (
	orgField = ".spec.organization"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=paradox.macro.re,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=paradox.macro.re,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=paradox.macro.re,resources=buckets/finalizers,verbs=update

//+kubebuilder:rbac:groups=paradox.macro.re,resources=organizations,verbs=get
//+kubebuilder:rbac:groups=paradox.macro.re,resources=organizations/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var bucket paradoxv1alpha1.Bucket
	if err := r.Get(ctx, req.NamespacedName, &bucket); err != nil {
		log.Error(err, "unable to fetch bucket")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("bucket", bucket)

	var organization paradoxv1alpha1.Organization
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.NamespacedName.Namespace,
		Name:      bucket.Spec.Organization,
	}, &organization); err != nil {
		log.Error(err, "unable to fetch organization")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	status := paradoxv1alpha1.BucketStatus{
		Instances: paradoxv1alpha1.Instances{},
	}

	if err := forEachInstanceClient(ctx, r.Client, &organization, func(instance *paradoxv1alpha1.Instance, client influxdb.Client) error {
		namespace, name := instance.ObjectMeta.Namespace, instance.ObjectMeta.Name
		wrapErr := func(err error) error {
			return fmt.Errorf("influx instance '%s/%s': %w", namespace, name, err)
		}

		bucketAPI := client.BucketsAPI()
		bkt, err := bucketAPI.FindBucketByName(ctx, bucket.Spec.Name)
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return wrapErr(err)
			}

			orgInstance := organization.Status.Instances[namespace][name]

			// create bucket if not exists

			bkt, err = bucketAPI.CreateBucket(ctx, domainBucket(orgInstance.ID, bucket))
			if err != nil {
				return wrapErr(err)
			}

			status.Instances.AddInstance(
				instance,
				fromStringPtr[paradoxv1alpha1.InfluxID](bkt.Id),
			)

			return nil
		}

		// update bucket if it exists and differs

		if bkt.Description == nil || *bkt.Description != bucket.Spec.Description {
			bkt.Description = &bucket.Spec.Description
			bkt, err = bucketAPI.UpdateBucket(ctx, bkt)
			if err != nil {
				return wrapErr(err)
			}
		}

		status.Instances.AddInstance(
			instance,
			fromStringPtr[paradoxv1alpha1.InfluxID](bkt.Id),
		)

		return nil
	}); err != nil {
		log.Error(err, "error while configuring instances")

		return ctrl.Result{}, err
	}

	bucket.Status = status

	if err := r.Status().Update(ctx, &bucket); err != nil {
		log.Error(err, "failed to update status")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func domainBucket(orgID *paradoxv1alpha1.InfluxID, bucket paradoxv1alpha1.Bucket) *domain.Bucket {
	var rr domain.RetentionRules
	if bucket.Spec.RetentionPolicy != "" {
		dur, err := time.ParseDuration(bucket.Spec.RetentionPolicy)
		if err == nil {
			rr = domain.RetentionRules{
				domain.RetentionRule{
					EverySeconds: int64(dur.Seconds()),
				},
			}
		}
	}

	return &domain.Bucket{
		Name:           bucket.Spec.Name,
		OrgID:          toStringPtr(orgID),
		Description:    &bucket.Spec.Description,
		RetentionRules: rr,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &paradoxv1alpha1.Bucket{}, orgField, func(rawObj client.Object) []string {
		// Extract the ConfigMap name from the ConfigDeployment Spec, if one is provided
		bkt := rawObj.(*paradoxv1alpha1.Bucket)
		if bkt.Spec.Organization == "" {
			return nil
		}
		return []string{bkt.Spec.Organization}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&paradoxv1alpha1.Bucket{}).
		Watches(
			&source.Kind{Type: &paradoxv1alpha1.Organization{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForOrganization),
		).
		Complete(r)
}

func (r *BucketReconciler) findObjectsForOrganization(org client.Object) []reconcile.Request {
	associatedBuckets := &paradoxv1alpha1.BucketList{}
	if err := r.List(context.TODO(), associatedBuckets, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(orgField, org.GetName()),
		Namespace:     org.GetNamespace(),
	}); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(associatedBuckets.Items))
	for i, item := range associatedBuckets.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}
