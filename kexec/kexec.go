/*
Copyright 2016 Xuan Tang. All rights reserved.
Use of this source code is governed by a license
that can be found in the LICENSE file.
*/

package main

import (
	"fmt"
	"time"

	"github.com/wayn3h0/go-uuid"
	"k8s.io/client-go/1.4/kubernetes"
	unversioned "k8s.io/client-go/1.4/pkg/api/unversioned"
	v1 "k8s.io/client-go/1.4/pkg/api/v1"
	batchv1 "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/tools/clientcmd"
)

func main() {
	c := &KexecConfig{
		KubeConfig: "./fakekubeconfig",
	}

	k, err := NewKexec(c)
	if err != nil {
		panic(err)
	}

	labels := make(map[string]string)
	labels["purpose"] = "benchmark"
	start := time.Now()
	for i := 0; i < 100; i++ {
		err = k.CallFunction("helloworld", "xuant/aceeditor", "default", labels)
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("Elapsed: %s", time.Since(start))
}

type KexecConfig struct {
	KubeConfig string
}

type Kexec struct {
	Clientset *kubernetes.Clientset
}

func NewKexec(c *KexecConfig) (*Kexec, error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Kexec{
		Clientset: clientset,
	}, nil
}

func (k *Kexec) CallFunction(function, image, namespace string, labels map[string]string) error {
	uuid, err := uuid.NewTimeBased()
	if err != nil {
		return err
	}
	jobname := function + "-" + uuid.String()
	fmt.Println(jobname)
	job := createJobFromTemplate(image, jobname, namespace, labels)

	_, err = k.Clientset.Batch().Jobs(namespace).Create(job)
	if err != nil {
		return err
	}
	return nil
}

func createJobFromTemplate(image, jobname, namespace string, labels map[string]string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      jobname,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Name: jobname,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:  jobname,
							Image: image,
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}
}
