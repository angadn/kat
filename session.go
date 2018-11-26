package kat

import (
	"fmt"
	"io"

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

func New(config *rest.Config, img Image) (session Session, err error) {
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
		case v1.PodSucceeded:
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
	var req *rest.Request
	req = session.clientset.RESTClient().Post().
		Resource("pods").
		Name(session.pod.Name).
		Namespace(string(session.NS)).
		SubResource("attach").
		VersionedParams(&v1.PodAttachOptions{
			Container: string(defaultContainer),
			Stdin:     true,
		}, scheme.ParameterCodec)

	var exec remotecommand.Executor
	if exec, err = remotecommand.NewSPDYExecutor(
		session.config, "POST", req.URL(),
	); err != nil {
		return
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})

	return
}
