package polkadot

import (
	"context"
	"github.com/go-logr/logr"
	polkadotv1alpha1 "github.com/swisscom-blockchain/polkadot-k8s-operator/pkg/apis/polkadot/v1alpha1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcilePolkadot) handleNetworkPolicy(CRInstance *polkadotv1alpha1.Polkadot) (bool, error) {

	if CRKind(CRInstance.Spec.Kind) == SentryAndValidator {
		return r.handleSpecificNetworkPolicy(CRInstance, newValidatorNetworkPolicyForCR(CRInstance))
	}

	return defaultHandler()
}

func (r *ReconcilePolkadot) handleSpecificNetworkPolicy(CRInstance *polkadotv1alpha1.Polkadot, desiredNetworkPolicy *v1.NetworkPolicy) (bool, error) {

	logger := log.WithValues("Service.Namespace", desiredNetworkPolicy.Namespace, "Service.Name", desiredNetworkPolicy.Name)

	foundNP, err := r.fetchNP(desiredNetworkPolicy)
	if err != nil {
		logger.Error(err, "Error on fetch the Network Policy...")
		return NotForcedRequeue, err
	}
	if foundNP == nil {
		logger.Info("Network Policy not found...")
		logger.Info("Creating a new Network Policy...")
		err := r.createNP(desiredNetworkPolicy, CRInstance, logger)
		if err != nil {
			logger.Error(err, "Error on creating a new Network Policy...")
			return NotForcedRequeue, err
		}
		logger.Info("Created the new Network Policy")
		return ForcedRequeue, nil
	}

	return NotForcedRequeue, nil
}

func (r *ReconcilePolkadot) fetchNP(np *v1.NetworkPolicy) (*v1.NetworkPolicy, error) {
	found := &v1.NetworkPolicy{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: np.Name, Namespace: np.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (r *ReconcilePolkadot) createNP(networkPolicy *v1.NetworkPolicy, CRInstance *polkadotv1alpha1.Polkadot, logger logr.Logger) error {
	err := r.setOwnership(CRInstance, networkPolicy)
	if err != nil {
		logger.Error(err, "Error on setting the ownership...")
		return err
	}
	return r.client.Create(context.TODO(), networkPolicy)
}