package kat

import (
	"fmt"
	"io"
	"log"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/kubernetes/scheme"

	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type container string

const (
	defaultContainer = container("kat")

	bufSize = 32768
)

type Session struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
	img       Image

	NS         Namespace
	Env        map[string]string
	PullPolicy v1.PullPolicy

	pod *v1.Pod
}

func New(config *rest.Config, img Image) (session *Session, err error) {
	session = new(Session)
	session.config = config

	if session.clientset, err = kubernetes.NewForConfig(config); err != nil {
		return
	}

	session.NS = DefaultNS
	session.img = img
	session.PullPolicy = v1.PullAlways
	return
}

func (session *Session) Start() (err error) {
	podName := uuid.New().String()

	var env []v1.EnvVar
	for k, v := range session.Env {
		env = append(env, v1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	session.pod, err = session.clientset.CoreV1().Pods(string(session.NS)).
		Create(&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: string(session.NS),
				Labels: map[string]string{
					podName: podName,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{
						Name:            string(defaultContainer),
						Image:           string(session.img),
						ImagePullPolicy: session.PullPolicy,
						Stdin:           true,
						TTY:             true,
						Env:             env,
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		})

	var watch watch.Interface
	if watch, err = session.clientset.CoreV1().Pods(
		string(session.NS),
	).Watch(metav1.ListOptions{
		LabelSelector: session.pod.Name,
	}); err != nil {
		return
	}

	for event := range watch.ResultChan() {
		switch event.Object.(*v1.Pod).Status.Phase {
		case v1.PodFailed:
			err = fmt.Errorf("pod %s failed", session.pod.Name)
			return
		case v1.PodUnknown:
			err = fmt.Errorf("failed to connect to pod %s", session.pod.Name)
			return
		case v1.PodRunning, v1.PodSucceeded:
			return
		default:
			// Do nothing
		}
	}

	return
}

func (session *Session) Attach(
	stdin io.Reader, stdout, stderr io.Writer,
) (err error) {
	var (
		client *rest.RESTClient
		req    *rest.Request
	)

	// Setup GroupVersion and NegotiatedSerializer, without which RESTClientFor panics
	session.config.GroupVersion = &schema.GroupVersion{}
	if *session.config.GroupVersion, err = schema.ParseGroupVersion(
		"v1",
	); err != nil {
		log.Fatalf("error parsing group version: %s\n", err.Error())
	}

	session.config.NegotiatedSerializer = scheme.Codecs

	// Make RESTClient
	if client, err = rest.RESTClientFor(session.config); err != nil {
		return
	}

	// Perform request
	req = client.Post().
		Resource("pods").
		Name(session.pod.Name).
		Namespace(string(session.NS)).
		SubResource("attach").
		VersionedParams(&v1.PodAttachOptions{
			Container: string(defaultContainer),
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	var exec remotecommand.Executor
	if exec, err = remotecommand.NewSPDYExecutor(
		session.config, "POST", req.URL(),
	); err != nil {
		return
	}

	log.Println(req.URL().String())

	/*
		XXX: returns error = "unable to upgrade connection: you must specify at least 1 of stdin, stdout, stderr" as req.URL() is missing stdin=true&stdout=true, etc. for `api/v1`. When GroupVersion is modified go `v1`, we see the params and an empty error.

		References:
		* https://docs.okd.io/latest/go_client/executing_remote_processes.html
		* https://github.com/a4abhishek/Client-Go-Examples/blob/master/exec_to_pod/exec_to_pod.go
		* https://github.com/kubernetes/kubernetes/blob/v1.6.1/test/e2e/framework/exec_util.go

	*/
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    true,
	})

	return
}
