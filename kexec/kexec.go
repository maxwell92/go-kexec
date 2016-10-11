/*
Copyright 2016 Xuan Tang. All rights reserved.
Use of this source code is governed by a license
that can be found in the LICENSE file.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/wayn3h0/go-uuid"
	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	unversioned "k8s.io/client-go/1.4/pkg/api/unversioned"
	v1 "k8s.io/client-go/1.4/pkg/api/v1"
	batchv1 "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/pkg/labels"
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

	/*
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
	*/
	funcLog, err := k.GetFunctionLog("helloworld", "a74a031b-8f23-11e6-80a2-b8e85639d46e", "default")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Function Log:\n %s", string(funcLog))

}

type LogType string

const (
	Completed LogType = "completed"
	Failed    LogType = "failed"
)

type FunctionLog struct {
	Timestamp time.Time
	Type      LogType
	Msg       string
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
	template := createJobTemplate(image, jobname, namespace, labels)

	_, err = k.Clientset.Batch().Jobs(namespace).Create(template)
	if err != nil {
		return err
	}
	return nil
}

func (k *Kexec) GetFunctionLog(funcName, uuidStr, namespace string) ([]byte, error) {
	podlist, err := k.getFunctionPods(funcName, uuidStr, namespace)
	if err != nil {
		return nil, err
	}

	if len(podlist.Items) < 1 {
		return nil, fmt.Errorf("No pod found for function %s - execution %s.", funcName, uuidStr)
	}

	// Only return the log of the first pod
	podName := podlist.Items[0].Name
	opts := &v1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}

	response, err := k.Clientset.Core().Pods(namespace).GetLogs(podName, opts).Stream()
	if err != nil {
		return nil, err
	}
	defer response.Close()

	return ioutil.ReadAll(response)

}

func (k *Kexec) GetFunctionPods(funcName, uuidStr, namespace string) (*v1.PodList, error) {
	return k.getFunctionPods(funcName, uuidStr, namespace)
}

func (k *Kexec) getFunctionPods(funcName, uuidStr, namespace string) (*v1.PodList, error) {
	// funcUUID is the label of the pod that ran the job
	funcUUID := funcName + "-" + uuidStr

	// Create job label selector
	jobLabelSelector := labels.SelectorFromSet(labels.Set{
		"job-name": funcUUID,
	})

	// List pods according to `jobLabelSelector`
	listOptions := api.ListOptions{
		LabelSelector: jobLabelSelector,
	}

	return k.Clientset.Core().Pods(namespace).List(listOptions)
}

func createJobTemplate(image, jobname, namespace string, labels map[string]string) *batchv1.Job {
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
