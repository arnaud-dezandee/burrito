package terragruntstack

import (
	"context"
	"fmt"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action string

const (
	PlanAction  Action = "plan"
	ApplyAction Action = "apply"
)

func GetDefaultLabels(stack *configv1alpha1.TerragruntStack) map[string]string {
	return map[string]string{
		"burrito/managed-by": stack.Name,
	}
}

func (r *Reconciler) getRun(stack *configv1alpha1.TerragruntStack, revision string, action Action) configv1alpha1.TerragruntStackRun {
	return configv1alpha1.TerragruntStackRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-%s-", stack.Name, action),
			Namespace:    stack.Namespace,
			Labels:       GetDefaultLabels(stack),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: stack.GetAPIVersion(),
					Kind:       stack.GetKind(),
					Name:       stack.Name,
					UID:        stack.UID,
				},
			},
		},
		Spec: configv1alpha1.TerragruntStackRunSpec{
			Action: string(action),
			Stack: configv1alpha1.TerragruntStackRunStack{
				Name:      stack.Name,
				Namespace: stack.Namespace,
				Revision:  revision,
			},
		},
	}
}

func (r *Reconciler) getAllRuns(ctx context.Context, stack *configv1alpha1.TerragruntStack) ([]*configv1alpha1.TerragruntStackRun, error) {
	list := &configv1alpha1.TerragruntStackRunList{}
	labelSelector := labels.NewSelector()
	for key, value := range GetDefaultLabels(stack) {
		requirement, err := labels.NewRequirement(key, selection.Equals, []string{value})
		if err != nil {
			return []*configv1alpha1.TerragruntStackRun{}, err
		}
		labelSelector = labelSelector.Add(*requirement)
	}
	err := r.Client.List(
		ctx,
		list,
		client.MatchingLabelsSelector{Selector: labelSelector},
		&client.ListOptions{Namespace: stack.Namespace},
	)
	if err != nil {
		return []*configv1alpha1.TerragruntStackRun{}, err
	}

	var runs []*configv1alpha1.TerragruntStackRun
	for _, run := range list.Items {
		runs = append(runs, &run)
	}
	return runs, nil
}

func deleteAll(ctx context.Context, c client.Client, objs []*configv1alpha1.TerragruntStackRun) error {
	for _, obj := range objs {
		if err := c.Delete(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
