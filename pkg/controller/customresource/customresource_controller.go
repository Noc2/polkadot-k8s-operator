package customresource

import (
	"context"
	"github.com/go-logr/logr"
	cachev1alpha1 "github.com/ironoa/kubernetes-customresource-operator/pkg/apis/cache/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_customresource")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new CustomResource Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCustomResource{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("customresource-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 1})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CustomResource
	err = c.Watch(&source.Kind{Type: &cachev1alpha1.CustomResource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Deployments and requeue the owner CustomResource
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cachev1alpha1.CustomResource{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cachev1alpha1.CustomResource{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCustomResource implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCustomResource{}

// ReconcileCustomResource reconciles a CustomResource object
type ReconcileCustomResource struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a CustomResource object and makes changes based on the state read
// and what is in the CustomResource.Spec
func (r *ReconcileCustomResource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.Info("Reconciling CustomResource")

	handledCRInstance, err := r.handleCustomResource(request)
	if err != nil {
		return handleRequeueError(err,logger)
	}
	if handledCRInstance == nil {
		return handleRequeueStd(err, logger)
	}

	isRequeueForced, err := r.handleDeployment(handledCRInstance)
	if err != nil {
		return handleRequeueError(err,logger)
	}
	if isRequeueForced {
		return handleRequeueForced(err, logger)
	}

	isRequeueForced, err = r.handleService(handledCRInstance)
	if err != nil {
		return handleRequeueError(err,logger)
	}
	if isRequeueForced {
		return handleRequeueForced(err, logger)
	}

	isRequeueForced, err = r.handlePVC(handledCRInstance)
	if err != nil {
		return handleRequeueError(err,logger)
	}
	if isRequeueForced {
		return handleRequeueForced(err, logger)
	}

	return handleRequeueStd(err, logger)
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func handleRequeueError (err error, logger logr.Logger) (reconcile.Result, error){
	logger.Info("Requeing the Reconciling request... ")
	return reconcile.Result{}, err
}

func handleRequeueForced (err error, logger logr.Logger) (reconcile.Result, error){
	logger.Info("Requeing the Reconciling request... ")
	return reconcile.Result{Requeue: true}, nil
}

func handleRequeueStd (err error, logger logr.Logger) (reconcile.Result, error){
	logger.Info("Return and not requeing the request")
	return reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileCustomResource) setOwnership(owner metav1.Object, owned metav1.Object) error {
	return controllerutil.SetControllerReference(owner, owned, r.scheme)
}

func (r *ReconcileCustomResource) handleCustomResource(request reconcile.Request) (*cachev1alpha1.CustomResource, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	found, err := r.fetchCustomResource(request)
	if err != nil {
		logger.Error(err, "Error on fetch the Custom Resource...")
		return nil, err
	}
	if found == nil {
		logger.Info("Custom Resource not found...")
		return nil, nil
	}

	return found, nil
}

func (r *ReconcileCustomResource) fetchCustomResource(request reconcile.Request) (*cachev1alpha1.CustomResource, error) {
	found := &cachev1alpha1.CustomResource{}
	err := r.client.Get(context.TODO(), request.NamespacedName, found)
	if err != nil && errors.IsNotFound(err) {
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		return nil, nil
	}
	return found, err
}

func (r *ReconcileCustomResource) handleDeployment(CRInstance *cachev1alpha1.CustomResource) (bool, error) {
	const NotForcedRequeue = false
	const ForcedRequeue = true

	desiredDeployment := newDeploymentForCR(CRInstance)
	logger := log.WithValues("Deployment.Namespace", desiredDeployment.Namespace, "Deployment.Name", desiredDeployment.Name)

	foundDeployment, err := r.fetchDeployment(desiredDeployment)
	if err != nil {
		logger.Error(err, "Error on fetch the Deployment...")
		return NotForcedRequeue, err
	}
	if foundDeployment == nil {
		logger.Info("Deployment not found...")
		err := r.createDeployment(desiredDeployment, CRInstance, logger)
		if err != nil {
			logger.Error(err, "Error on creating a new Deployment...")
			return NotForcedRequeue, err
		}
		logger.Info("Created the new Deployment")
		return ForcedRequeue, nil
	}

	if areDeploymentsDifferent(foundDeployment, desiredDeployment, logger) {
		err := r.updateDeployment(desiredDeployment, logger)
		if err != nil {
			logger.Error(err, "Update Deployment Error...")
			return NotForcedRequeue, err
		}
		logger.Info("Updated the Deployment...")
	}

	return NotForcedRequeue, nil
}

func (r *ReconcileCustomResource) fetchDeployment(deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (r *ReconcileCustomResource) createDeployment(deployment *appsv1.Deployment, CRInstance *cachev1alpha1.CustomResource, logger logr.Logger) error {
	logger.Info("Creating a new Deployment...")
	err := r.setOwnership(CRInstance, deployment)
	if err != nil {
		logger.Error(err, "Error on setting the ownership...")
		return err
	}
	err = r.client.Create(context.TODO(), deployment)
	return err
}

func areDeploymentsDifferent(currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment, logger logr.Logger) bool {
	result := false

	if isDeploymentReplicaDifferent(currentDeployment, desiredDeployment, logger) {
		result = true
	}
	if isDeploymentVersionDifferent(currentDeployment, desiredDeployment, logger) {
		result = true
	}

	return result
}

func isDeploymentReplicaDifferent(currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment, logger logr.Logger) bool {
	size := *desiredDeployment.Spec.Replicas
	if *currentDeployment.Spec.Replicas != size {
		logger.Info("Find a replica size mismatch...")
		return true
	}
	return false
}

func isDeploymentVersionDifferent(currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment, logger logr.Logger) bool {
	version := desiredDeployment.ObjectMeta.Labels["version"]
	if currentDeployment.ObjectMeta.Labels["version"] != version {
		logger.Info("Found a version mismatch...")
		return true
	}
	return false
}

func (r *ReconcileCustomResource) updateDeployment(deployment *appsv1.Deployment, logger logr.Logger) error {
	logger.Info("Updating the Deployment...")
	return r.client.Update(context.TODO(), deployment)
}

func (r *ReconcileCustomResource) handleService(CRInstance *cachev1alpha1.CustomResource) (bool, error) {
	const NotForcedRequeue = false
	const ForcedRequeue = true

	desiredService := newServiceForCR(CRInstance)
	logger := log.WithValues("Service.Namespace", desiredService.Namespace, "Service.Name", desiredService.Name)

	foundService, err := r.fetchService(desiredService)
	if err != nil {
		logger.Error(err, "Error on fetch the Service...")
		return NotForcedRequeue, err
	}
	if foundService == nil {
		logger.Info("Service not found...")
		err := r.createService(desiredService, CRInstance, logger)
		if err != nil {
			logger.Error(err, "Error on creating a new Service...")
			return NotForcedRequeue, err
		}
		logger.Info("Created the new Service")
		return ForcedRequeue, nil
	}

	if areServicesDifferent(foundService, desiredService, logger) {
		err := r.updateService(desiredService, logger)
		if err != nil {
			logger.Error(err, "Update Service Error...")
			return NotForcedRequeue, err
		}
		logger.Info("Updated the Service...")
	}

	return NotForcedRequeue, nil
}

func (r *ReconcileCustomResource) fetchService(service *corev1.Service) (*corev1.Service, error) {
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (r *ReconcileCustomResource) createService(service *corev1.Service, CRInstance *cachev1alpha1.CustomResource, logger logr.Logger) error {
	logger.Info("Creating a new Service...")
	err := r.setOwnership(CRInstance, service)
	if err != nil {
		logger.Error(err, "Error on setting the ownership...")
		return err
	}
	return r.client.Create(context.TODO(), service)
}

func areServicesDifferent(currentService *corev1.Service, desiredService *corev1.Service, logger logr.Logger) bool {
	result := false
	return result
}

func (r *ReconcileCustomResource) updateService(service *corev1.Service, logger logr.Logger) error {
	logger.Info("Updating the Service...")
	return r.client.Update(context.TODO(), service)
}

func (r *ReconcileCustomResource) handlePVC(CRInstance *cachev1alpha1.CustomResource) (bool, error) {
	const NotForcedRequeue = false
	const ForcedRequeue = true

	desiredPVC := newPVCForCR(CRInstance)
	logger := log.WithValues("PVC.Namespace", desiredPVC.Namespace, "PVC.Name", desiredPVC.Name)

	foundPVC, err := r.fetchPVC(desiredPVC)
	if err != nil {
		logger.Error(err, "Error on fetch the PVC...")
		return NotForcedRequeue, err
	}
	if foundPVC == nil {
		logger.Info("PVC not found...")
		err := r.createPVC(desiredPVC, CRInstance, logger)
		if err != nil {
			logger.Error(err, "Error on creating a new PVC...")
			return NotForcedRequeue, err
		}
		logger.Info("Created the new PVC")
		return ForcedRequeue, nil
	}

	if arePVCsDifferent(foundPVC, desiredPVC, logger) {
		err := r.updatePVC(desiredPVC, logger)
		if err != nil {
			logger.Error(err, "Update PVC Error...")
			return NotForcedRequeue, err
		}
		logger.Info("Updated the PVC...")
	}

	return NotForcedRequeue, nil
}

func (r *ReconcileCustomResource) fetchPVC(PVC *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	found := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: PVC.Name, Namespace: PVC.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (r *ReconcileCustomResource) createPVC(PVC *corev1.PersistentVolumeClaim, CRInstance *cachev1alpha1.CustomResource, logger logr.Logger) error {
	logger.Info("Creating a new PVC...")
	err := r.setOwnership(CRInstance, PVC)
	if err != nil {
		logger.Error(err, "Error on setting the ownership...")
		return err
	}
	return r.client.Create(context.TODO(), PVC)
}

func arePVCsDifferent(currentPVC *corev1.PersistentVolumeClaim, desiredPVC *corev1.PersistentVolumeClaim, logger logr.Logger) bool {
	result := false
	return result
}

func (r *ReconcileCustomResource) updatePVC(PVC *corev1.PersistentVolumeClaim, logger logr.Logger) error {
	logger.Info("Updating the Persistent Volume Claim...")
	return r.client.Update(context.TODO(), PVC)
}

func newDeploymentForCR(CRInstance *cachev1alpha1.CustomResource) *appsv1.Deployment {
	labels := labelsForApp(CRInstance)
	replicas := CRInstance.Spec.Size
	version := CRInstance.Spec.Version
	labelsWithVersion := labelsForAppWithVersion(CRInstance, version)
	volumeName := "polkadot-data"

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRInstance.Name + "-deployment",
			Namespace: CRInstance.Namespace,
			Labels:    labelsWithVersion,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name:         volumeName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: getPVCName(CRInstance),
							},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "polkadot",
						Image: "chevdor/polkadot:" + version,
						VolumeMounts: []corev1.VolumeMount{{
							Name: volumeName,
							MountPath: "/data",
						}},
						Command: []string{
							"polkadot", "--name", "Ironoa", "--rpc-external", "--rpc-cors=all",
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 30333,
							},
							{
								ContainerPort: 9933,
							},
							{
								ContainerPort: 9944,
							},
						},
					}},
				},
			},
		},
	}
}

func newServiceForCR(CRInstance *cachev1alpha1.CustomResource) *corev1.Service {
	labels := labelsForApp(CRInstance)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRInstance.Name + "-service",
			Namespace: CRInstance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:       "to-be-defined-a",
					Port:       30333,
					TargetPort: intstr.FromInt(30333),
					Protocol:   "TCP",
				},
				{
					Name:       "to-be-defined-b",
					Port:       9933,
					TargetPort: intstr.FromInt(9933),
					Protocol:   "TCP",
				},
				{
					Name:       "to-be-defined-c",
					Port:       9944,
					TargetPort: intstr.FromInt(9944),
					Protocol:   "TCP",
				},
			},
			Selector: labels,
		},
	}
}

func newPVCForCR(CRInstance *cachev1alpha1.CustomResource) *corev1.PersistentVolumeClaim {
	labels := labelsForApp(CRInstance)
	storageClassName := "polkadot"
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPVCName(CRInstance),
			Namespace: CRInstance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
			},
		},
	}
}

// labelsForApp creates a simple set of labels for App.
func labelsForApp(cr *cachev1alpha1.CustomResource) map[string]string {
	return map[string]string{"app": cr.Name, "app_cr": cr.Name}
}

func labelsForAppWithVersion(cr *cachev1alpha1.CustomResource, version string) map[string]string {
	labels := labelsForApp(cr)
	labels["version"] = version
	return labels
}

func matchingLabels(cr *cachev1alpha1.CustomResource) map[string]string {
	return map[string]string{
		"app":    cr.Name,
		"server": cr.Name,
	}
}

func serverLabels(cr *cachev1alpha1.CustomResource) map[string]string {
	labels := map[string]string{
		"version": cr.Spec.Version,
	}
	for k, v := range matchingLabels(cr) {
		labels[k] = v
	}
	return labels
}

func getPVCName(CRInstance *cachev1alpha1.CustomResource) string {
	return CRInstance.Name + "-pvc"
}
