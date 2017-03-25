package rbac

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/rbac/v1beta1"
)

func GenerateRoleByNamespace(namespace string) *v1beta1.Role {
	return nil
}

func GenerateRoleBindingByNamespace(namespace string) *v1beta1.RoleBinding {
	return nil
}

func GenerateClusterRole() *v1beta1.ClusterRole {
	policyRule := v1beta1.PolicyRule{
		Verbs:     []string{v1beta1.VerbAll},
		APIGroups: []string{v1beta1.APIGroupAll},
		Resources: []string{"namespaces"},
	}

	clusterRole := &v1beta1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-creater",
		},
		Rules: []v1beta1.PolicyRule{policyRule},
	}
	return clusterRole
}

func GenerateClusterRoleBindingByTenant(tenant string) *v1beta1.ClusterRoleBinding {
	subject := v1beta1.Subject{
		Kind: "User",
		Name: tenant,
	}
	roleRef := v1beta1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "namespace-creater",
	}

	clusterRoleBinding := &v1beta1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "clusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tenant + "-create-namespace",
		},
		Subjects: []v1beta1.Subject{subject},
		RoleRef:  roleRef,
	}
	return clusterRoleBinding
}
