package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureNamespace creates a namespace if it doesn't exist
func (c *Client) EnsureNamespace(ctx context.Context, namespace string) error {
	c.logger.Infof("Ensuring namespace %s exists", namespace)

	// Check if namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		c.logger.Infof("✓ Namespace %s already exists", namespace)
		return nil
	}

	// If error is not "not found", return it
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace: %w", err)
	}

	// Create namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"name":                         namespace,
				"app.kubernetes.io/name":       "lumo",
				"app.kubernetes.io/managed-by": "lumo-deploy",
			},
		},
	}

	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	c.logger.Infof("✓ Namespace %s created", namespace)

	return nil
}

// DeleteNamespace deletes a namespace
func (c *Client) DeleteNamespace(ctx context.Context, namespace string) error {
	c.logger.Infof("Deleting namespace %s", namespace)

	err := c.clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.logger.Infof("Namespace %s does not exist, skipping deletion", namespace)
			return nil
		}
		return fmt.Errorf("delete namespace: %w", err)
	}

	c.logger.Infof("✓ Namespace %s deleted", namespace)

	return nil
}

// NamespaceExists checks if a namespace exists
func (c *Client) NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get namespace: %w", err)
	}

	return true, nil
}
