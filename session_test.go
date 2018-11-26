package kat

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func TestFoo(t *testing.T) {
	var (
		err     error
		config  *rest.Config
		session *Session
	)

	if config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(
		os.Getenv("HOME"), ".kube", "config",
	)); err != nil {
		log.Fatalf("error getting InClusterConfig: %s\n", err.Error())
	}

	if session, err = New(config, Image("angadn/cat")); err != nil {
		log.Fatalf("error creating Session: %s\n", err.Error())
	}

	log.Printf("starting...\n")
	if err = session.Start(); err != nil {
		log.Fatalf("error starting Session: %s\n", err.Error())
	}

	var (
		stdin          io.Reader
		stdout, stderr *bytes.Buffer
	)

	stdin = strings.NewReader("hello, world")
	stdout = bytes.NewBuffer(make([]byte, 32768))
	stderr = bytes.NewBuffer(make([]byte, 32767))

	log.Printf("attaching to '%s'...\n", session.pod.Name)
	if err = session.Attach(stdin, stdout, stderr); err != nil {
		log.Fatalf("error attaching to pod %s: %s\n", session.pod.Name, err.Error())
	}

	log.Printf("Output: %s\n", stdout.String())
	log.Printf("Errors: %s\n", stderr.String())
}
