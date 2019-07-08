This is not an officially supported Google product

This is a demo for the Kubernetes API Deep Dive talk at O'Reilly Software Architecture 2019

A fully-implemented version of this demo can be found on the
[solutions/software-architecture-2019](https://github.com/DirectXMan12/kubebuilder-workshops/tree/solutions/software-architecture-2019)
branch for reference.

# Kubebuilder Workshop

The Kubebuilder Workshop provides a hands-on experience creating Kubernetes APIs using kubebuilder.

Futher documentation on kubebuilder can be found [here](https://book.kubebuilder.io/)

Create a Kubernetes API for creating MongoDB instances similar to what is shown in
[this blog post](https://kubernetes.io/blog/2017/01/running-mongodb-on-kubernetes-with-statefulsets/)

It should be possible to manage MongoDB instances by running `kubectl apply -f` on the yaml file
declaring a MongoDB instance.

```yaml
apiVersion: databases.k8s.io/v1alpha1
kind: MongoDB
metadata:
  name: mongo-instance
spec:
  replicas: 3
  storage: 100Gi
```

## Kubernetes API Deep Dive Slides

[Posted Here](https://bit.ly/2JOn702)

## **Prerequisites**

- Provision a [kubernetes cluster](https://cloud.google.com/kubernetes-engine/)
- Install [go](https://golang.org/)
- Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- Install [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
  
## Setup the project

```bash
$ go mod init github.com/pwittrock/kubebuilder-workshop
$ kubebuilder init --domain example.com --license apache2 --owner "The Kubernetes authors"
```

### Create the MongoDB Resource and Controller Stubs

```bash
$ kubebuilder create api --group databases --version v1alpha1 --kind MongoDB
```

- enter `y` to have it create the stub for the Resource
- enter `y` to have it create the stub for the Controller

## Implement the Resource

### Update the MongoDBSpec

Modify the MongoDB API Schema (e.g. *MongoDBSpec*) in `api/v1alpha1/mongodb_types.go`.

```go
// MongoDBSpec defines the desired state of MongoDB
type MongoDBSpec struct {
	// replicas is the number of MongoDB replicas
    // +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

    // storage is the volume size for each instance
	// +optional
	Storage *string `json:"storage,omitempty"`	
}
```

### Update the MongoDBStatus

```go
// MongoDBStatus defines the observed state of MongoDB
type MongoDBStatus struct {
	// statefulSetStatus contains the status of the StatefulSet managed by MongoDB
	StatefulSetStatus appsv1.StatefulSetStatus `json:"statefulSetStatus,omitempty"`

	// serviceStatus contains the status of the Service managed by MongoDB
	ServiceStatus corev1.ServiceStatus `json:"serviceStatus,omitempty"`

	ClusterIP string `json:"clusterIP,omitempty"`
}
```

### Update the print column markers

These tell kubectl how to print the object.

```go
// +kubebuilder:printcolumn:name="storage",type="string",JSONPath=".spec.storage",format="byte"
// +kubebuilder:printcolumn:name="replicas",type="integer",JSONPath=".spec.replicas",format="int32"
// +kubebuilder:printcolumn:name="ready replicas",type="integer",JSONPath=".status.statefulSetStatus.readyReplicas",format="int32"
// +kubebuilder:printcolumn:name="current replicas",type="integer",JSONPath=".status.statefulSetStatus.currentReplicas",format="int32"
// +kubebuilder:printcolumn:name="cluster-ip",type="string",JSONPath=".status.clusterIP",format="byte"
```

### Add the status subresource

This allows status to be updated independently from the spec.

```go
// +kubebuilder:subresource:status
```

### Add the scale subresource

This allows `kubectl scale` work.

```go
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.statefulSetStatus.replicas
```

## Implement the Controller

### Read MongoDB from Reconcile

```go
	// Fetch the MongoDB instance
	mongo := &databasesv1alpha1.MongoDB{}
	if err := r.Get(ctx, req.NamespacedName, mongo); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
```

### Write the Service object

```go
    //
	// Generate Service object managed by MongoDB
	//
	
	// Init Service struct to create or update
	service := &corev1.Service{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name + "-mongodb-service",
			Namespace: req.Namespace,
		},
	}
	// Either create or update the Service
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, service, func() error {
        // Set the Service fields to the correct values
		util.SetServiceFields(service, mongo)

		// Add the owners reference to the MongoDB instance so the Service gets garbage collected
		return controllerutil.SetControllerReference(mongo, service, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
```

### Write the StatefulSet object

```go
    //
	// Generate StatefulSet managed by MongoDB
	//
	
	// Init StatefulSet struct to create or update
	stateful := &appsv1.StatefulSet{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name + "-mongodb-statefulset",
			Namespace: req.Namespace,
		},
	}
	// Either create or update the StatefulSet
	_, err = ctrl.CreateOrUpdate(ctx, r.Client, stateful, func() error {
        // Set the StatefulSet fields to the correct values
		util.SetStatefulSetFields(stateful, service, mongo, mongo.Spec.Replicas, mongo.Spec.Storage)

		// Add the owners reference to the MongoDB instance so the StatefulSet gets garbage collected
		return controllerutil.SetControllerReference(mongo, stateful, r.Scheme)

	})
	if err != nil {
		return ctrl.Result{}, err
	}
```

### Update the MongoDB status

```go
    //
    // Update the MongoDB status so it gets published to users
    //
    
	mongo.Status.StatefulSetStatus = stateful.Status
	mongo.Status.ServiceStatus = service.Status
	mongo.Status.ClusterIP = service.Spec.ClusterIP

    // Update the MongoDB status field using the "status" subresource
	err = r.Status().Update(ctx, mongo)
	if err != nil {
		return ctrl.Result{}, err
	}
```

### Watch StatefulSets and Services

Update the `SetupWithManager` function to configure MondoDB to own StatefulSets and Services.

```go
func (r *MongoDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MongoDB{}).
		Owns(&appsv1.StatefulSet{}). // Generates StatefulSets
		Owns(&corev1.Service{}).     // Generates Services
		Complete(r)
}
```

### Add StatefulSet and Service RBAC markers

Add an RBAC rule so the Controller can read / write StatuefulSets and Services

```go
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
```

## Update main.go

### Add StatefulSets and Services to the Scheme

This is necessary to read / write the objects from the client

```go
func init() {
	appsv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	databasesv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}
```

### Pass the Scheme and Recorder to the Controller

This will be needed later for setting owners references

main.go

```go
	err = (&controllers.MongoDBReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("MongoDB"),
		Recorder: mgr.GetEventRecorderFor("mongodb"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MongoDB")
		os.Exit(1)
	}
```

mongodb_controller.go

```go
type MongoDBReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}
```

## Run the API

### Install CRDs into a cluster

```bash
$ make manifests
$ kubectl apply -k config/crd
```

Nothing has been actuated yet:

```go
$ kubectl get mongodbs,statefulsets,services,pods
```


### Run the Controller locally

**Note:** Update `main.go` to disable leader election for running locally.

```bash
$ make run
```

### Edit the sample MongoDB file

Edit `config/samples/databases_v1alpha1_mongodb.yaml`

```yaml
apiVersion: databases.example.com/v1alpha1
kind: MongoDB
metadata:
  name: mongodb-sample
spec:
  replicas: 1
  storage: 100Gi
```

### Create the sample

```bash
$ kubectl apply -f config/samples/databases_v1alpha1_mongodb.yaml
```

## Test the Application

### Inspect cluster state

```go
$ kubectl get mongodbs,statefulsets,services,pods
$ kubectl logs mongodb-sample-mongodb-statefulset-0 mongo
```

### Connect to the running MongoDB instance from within the cluster using a Pod

```bash
$ kubectl run mongo-test -t -i --rm --image mongo bash
$ mongo <cluster ip address of mongodb service>:27017
```

### Test scale

```bash
$ rm -rf ~/.kube/cache/ # clear the discovery cache
$ kubectl scale mongodbs/mongodb-sample --replicas=3
$ kubectl get mongodbs 
```

## Test Garbage Collection

```bash
$ kubectl delete -f config/samples/databases_v1alpha1_mongodb.yaml
$ kubectl get mongodbs,statefulsets,services,pods
```

## Publishing and running in-cluster

Give yourself cluster permissions:

```bash
# install docker plugin for gcr.io
$ gcloud components install docker-credential-gcr
$ docker-credential-gcr configure-docker

# add cluster-admin permission
$ kubectl create clusterrolebinding kubebuilder-workshop-cluster-admin-binding --clusterrole cluster-admin --user $(gcloud config get-value account)

# build and push the image
$ docker build . -t gcr.io/<project>/kubebuilder-workshop:v1
$ docker push gcr.io/<project>/kubebuilder-workshop:v1
```

**Note:** Modify the Dockerfile to copy new package dirs

```bash
COPY util/ util/
```

Update `config/default/manager_image_patch.yaml` with the new image name

```bash
kubectl apply -k  config/default/
```
