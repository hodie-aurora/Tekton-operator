/*
Copyright 2023 The Tekton Authors

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
package result

import (
	"context"
	"testing"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestEnsureTektonResultExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tt := GetTektonResultCR(getTektonConfig(), "v0.70.0")

	// first invocation should create instance as it is non-existent and return RECONCILE_AGAIN_ERR
	_, err := EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// during second invocation instance exists but waiting on dependencies (pipeline, results)
	// hence returns RECONCILE_AGAIN_ERR
	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// make upgrade checks pass
	makeUpgradeCheckPass(t, ctx, c.OperatorV1alpha1().TektonResults())

	// next invocation should return RECONCILE_AGAIN_ERR as Dashboard is waiting for installation (prereconcile, postreconcile, installersets...)
	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the instance ready
	markResultReady(t, ctx, c.OperatorV1alpha1().TektonResults())

	// next invocation should return nil error as the instance is ready
	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, nil)

	// test update propagation from tektonConfig
	tt.Spec.TargetNamespace = "foobar"
	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, nil)
}

func TestEnsureTektonResultCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	// when no instance exists, nil error is returned immediately
	err := EnsureTektonResultCRNotExists(ctx, c.OperatorV1alpha1().TektonResults())
	util.AssertEqual(t, err, nil)

	// create an instance for testing other cases
	tt := GetTektonResultCR(getTektonConfig(), "v0.70.0")
	_, err = EnsureTektonResultExists(ctx, c.OperatorV1alpha1().TektonResults(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when an instance exists the first invocation should make the delete API call and
	// return RECONCILE_AGAIN_ERROR. So that the deletion can be confirmed in a subsequent invocation
	err = EnsureTektonResultCRNotExists(ctx, c.OperatorV1alpha1().TektonResults())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when the instance is completely removed from a cluster, the function should return nil error
	err = EnsureTektonResultCRNotExists(ctx, c.OperatorV1alpha1().TektonResults())
	util.AssertEqual(t, err, nil)
}

func markResultReady(t *testing.T, ctx context.Context, c op.TektonResultInterface) {
	t.Helper()
	tr, err := c.Get(ctx, v1alpha1.ResultResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	tr.Status.MarkDependenciesInstalled()
	tr.Status.MarkPreReconcilerComplete()
	tr.Status.MarkInstallerSetAvailable()
	tr.Status.MarkInstallerSetReady()
	tr.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, tr, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func makeUpgradeCheckPass(t *testing.T, ctx context.Context, c op.TektonResultInterface) {
	t.Helper()
	// set necessary version labels to make upgrade check pass
	result, err := c.Get(ctx, v1alpha1.ResultResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	setDummyVersionLabel(t, result)
	_, err = c.Update(ctx, result, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func setDummyVersionLabel(t *testing.T, tr *v1alpha1.TektonResult) {
	t.Helper()

	oprVersion := "v1.2.3"
	t.Setenv(v1alpha1.VersionEnvKey, oprVersion)

	labels := tr.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = oprVersion
	tr.SetLabels(labels)
}

func getTektonConfig() *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: v1alpha1.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
}
