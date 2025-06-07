/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/david/rollout-operator/api/v1"
	kapps "k8s.io/api/apps/v1"
)

// RolloutAppReconciler reconciles a RolloutApp object
type RolloutAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RolloutApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *RolloutAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. 获取 RolloutApp 对象
	var rollout appsv1.RolloutApp
	if err := r.Get(ctx, req.NamespacedName, &rollout); err != nil {
		if apierrors.IsNotFound(err) {
			// 已删除，无需处理
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch RolloutApp")
		return ctrl.Result{}, err
	}
	// 2. 获取目标 StatefulSet
	var sts kapps.StatefulSet
	err := r.Get(ctx, types.NamespacedName{
		Name:      rollout.Spec.StatefulSetName,
		Namespace: rollout.Spec.Namespace,
	}, &sts)
	if err != nil {
		log.Error(err, "unable to fetch StatefulSet")
		return ctrl.Result{}, err
	}

	// 3. TODO: 在这里添加检查 Image、更新 RollingUpdate.Partition 等逻辑
	log.Info("Fetched StatefulSet successfully", "name", sts.Name)

	// pre-check
	// wait 30s before updating partition
	log.Info("waiting 30 seconds before next partition step")
	time.Sleep(30 * time.Second)

	// begin controlled rollout
	partition := int32(0)
	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = *sts.Spec.UpdateStrategy.RollingUpdate.Partition
	}

	if partition == 0 {
		log.Info("rollout already complete")
		return ctrl.Result{}, nil
	}

	// step: decrease partition by 1
	newPartition := partition - 1
	sts.Spec.UpdateStrategy.RollingUpdate.Partition = &newPartition
	if err := r.Update(ctx, &sts); err != nil {
		log.Error(err, "failed to update StatefulSet partition")
		return ctrl.Result{}, err
	}
	log.Info("Updated partition", "partition", newPartition)

	// update status
	rollout.Status.CurrentPartition = newPartition
	rollout.Status.LastUpdated = time.Now().Format(time.RFC3339)
	rollout.Status.Phase = fmt.Sprintf("step-%d", newPartition)
	if err := r.Status().Update(ctx, &rollout); err != nil {
		log.Error(err, "failed to update RolloutApp status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RolloutAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.RolloutApp{}).
		Named("rolloutapp").
		Complete(r)
}
