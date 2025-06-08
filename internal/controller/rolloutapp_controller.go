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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/david/rollout-operator/api/v1"
	kapps "k8s.io/api/apps/v1"
)

// RolloutAppReconciler reconciles a RolloutApp object
type RolloutAppReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder // 添加这行
}

// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.weex.com,resources=rolloutapps/finalizers,verbs=update
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
	log.Info("Fetched StatefulSet successfully", "name", sts.Name,
		"suspend", rollout.Spec.Suspend, "partition", sts.Spec.UpdateStrategy.RollingUpdate.Partition)

	// begin controlled rollout
	partition := int32(0)
	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = *sts.Spec.UpdateStrategy.RollingUpdate.Partition
	}

	if rollout.Spec.Suspend {
		log.Info("Rollout suspended; skipping partition update")
		r.Recorder.Event(&rollout, "Normal", "RolloutSuspended", "Rollout is currently suspended; skipping partition update")

		return ctrl.Result{}, nil
	}

	if partition > 0 {
		log.Info("Decreasing partition")
		r.Recorder.Event(&rollout, "Normal", "PartitionStep", fmt.Sprintf("Partition decremented to %d", partition-1))
		// pre-check
		// wait 30s before updating partition
		log.Info("waiting 30 seconds, to add raft checking logic later")
		time.Sleep(10 * time.Second)

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

		return r.updateStatus(ctx, &rollout)
	}

	if partition == 0 && isUpdateComplete(&sts) && rollout.Status.Phase != "done" {
		log.Info("Rollout complete, resetting partition")
		r.Recorder.Event(&rollout, "Normal", "RolloutComplete", "All pods updated; resetting partition and suspending rollout")
		// reset to initial state after rollout is fully complete
		resetPartition := rollout.Spec.InitPartition
		sts.Spec.UpdateStrategy.RollingUpdate.Partition = &resetPartition
		if err := r.Update(ctx, &sts); err != nil {
			log.Error(err, "failed to reset partition to 3 after full rollout")
			return ctrl.Result{}, err
		}
		log.Info("Partition reset to 3 after full rollout")

		rollout.Spec.Suspend = true
		if err := r.Update(ctx, &rollout); err != nil {
			log.Error(err, "failed to update  after resetting partition")
			return ctrl.Result{}, err
		}

		// 再重新获取对象，保证 resourceVersion 是新的
		if err := r.Get(ctx, req.NamespacedName, &rollout); err != nil {
			return ctrl.Result{}, err
		}
		rollout.Status.CurrentPartition = resetPartition
		rollout.Status.Phase = "done"
		rollout.Status.LastUpdated = time.Now().Format(time.RFC3339)
		r.Recorder.Event(&rollout, "Normal", "ResetPartition", "Partition reset to InitPartition and rollout suspended")
		return r.updateStatus(ctx, &rollout)
	}

	// Waiting state
	log.Info("Waiting for rollout to complete")
	rollout.Status.Phase = "waiting-for-completion"
	rollout.Status.LastUpdated = time.Now().Format(time.RFC3339)
	return r.updateStatus(ctx, &rollout)

}

func (r *RolloutAppReconciler) updateStatus(ctx context.Context, rollout *appsv1.RolloutApp) (ctrl.Result, error) {
	if err := r.Status().Update(ctx, rollout); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}
func isUpdateComplete(sts *kapps.StatefulSet) bool {
	return sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
		sts.Status.UpdatedReplicas == *sts.Spec.Replicas
}

// SetupWithManager sets up the controller with the Manager.
func (r *RolloutAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("rolloutapp-controller") // 初始化 Recorder

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.RolloutApp{}).
		Named("rolloutapp").
		Complete(r)
}
