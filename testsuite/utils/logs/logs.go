package logs

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/Apicurio/apicurio-registry-k8s-tests-e2e/testsuite/utils"
	"github.com/Apicurio/apicurio-registry-k8s-tests-e2e/testsuite/utils/kubernetescli"
	"github.com/Apicurio/apicurio-registry-k8s-tests-e2e/testsuite/utils/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var log = logf.Log.WithName("logs")

func SaveOperatorLogs(clientset *kubernetes.Clientset, suiteID string, namespace string) {

	log.Info("Collecting operator logs", "suite", suiteID, "namespace", namespace)

	logsDir := utils.SuiteProjectDir + "/tests-logs/" + suiteID + "/operator/namespaces/" + namespace + "/"
	os.MkdirAll(logsDir, os.ModePerm)

	//first we collect all pods statuses and cluster events
	createPodsLogFile(logsDir, namespace)
	createEventsLogFile(logsDir, namespace)
	createNodesLogFile(logsDir)

	operatorDeployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), utils.OperatorDeploymentName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			log.Info("Skipping storing operator logs because operator deployment not found")
			return
		}
		Expect(err).ToNot(HaveOccurred())
	}
	if operatorDeployment.Status.AvailableReplicas == int32(0) {
		log.Info("Skipping storing operator logs because operator deployment is not ready")
		return
	}
	labelsSet := labels.Set(operatorDeployment.Spec.Selector.MatchLabels)
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelsSet.AsSelector().String()})
	Expect(err).ToNot(HaveOccurred())

	for _, pod := range pods.Items {
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
		podLogs, err := req.Stream(context.TODO())
		Expect(err).ToNot(HaveOccurred())
		defer podLogs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		Expect(err).ToNot(HaveOccurred())

		logFile := logsDir + pod.Name + ".log"
		log.Info("Storing operator logs", "file", logFile)
		err = ioutil.WriteFile(logFile, buf.Bytes(), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
	}
}

func PrintSeparator() {
	log.Info("-----------------------------------------------------------")
}

func SaveLogs(suiteCtx *types.SuiteContext, ctx *types.TestContext) {
	testDescription := CurrentSpecReport()

	testName := ""
	if len(testDescription.ContainerHierarchyTexts) != 0 {
		for _, comp := range testDescription.ContainerHierarchyTexts {
			testName += (comp + "-")
		}
		testName = testName[0 : len(testName)-1]
	}

	if ctx.ID != "" {
		if len(testName) == 0 {
			testName = ctx.ID
		} else {
			testName += ("-" + ctx.ID)
		}
	}

	SaveTestPodsLogs(suiteCtx.Clientset, suiteCtx.SuiteID, ctx.RegistryNamespace, testName)
}

//SaveTestPodsLogs stores logs of all pods in OperatorNamespace
func SaveTestPodsLogs(clientset *kubernetes.Clientset, suiteID string, namespace string, testName string) {

	log.Info("Collecting test logs", "suite", suiteID, "test", testName)

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())

	logsDir := utils.SuiteProjectDir + "/tests-logs/" + suiteID + "/" + testName + "/namespaces/" + namespace + "/"
	os.MkdirAll(logsDir, os.ModePerm)

	//first we collect all pods statuses and cluster events
	createPodsLogFile(logsDir, namespace)
	createEventsLogFile(logsDir, namespace)
	createNodesLogFile(logsDir)

	//then collect logs for each running pod
	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			log.Info("Skipping storing pod logs because pod is not ready", "pod", pod.Name)
			continue
		}
		for _, container := range pod.Status.ContainerStatuses {
			saveContainerLogs(clientset, logsDir, container.Name, pod)
		}
	}
}

func createPodsLogFile(logsDir string, namespace string) {
	currentPodsFile, err := os.Create(logsDir + "pods.log")
	Expect(err).ToNot(HaveOccurred())
	kubernetescli.RedirectOutput(currentPodsFile, os.Stderr, "get", "pods", "-n", namespace)
	kubernetescli.RedirectOutput(currentPodsFile, os.Stderr, "get", "pods", "-n", namespace, "-o", "yaml")
	defer currentPodsFile.Close()
}

func createEventsLogFile(logsDir string, namespace string) {
	eventsFile, err := os.Create(logsDir + "events.log")
	Expect(err).ToNot(HaveOccurred())
	kubernetescli.RedirectOutput(eventsFile, os.Stderr, "get", "events", "-n", namespace, "--sort-by=\"{.metadata.creationTimestamp}\"")
	defer eventsFile.Close()
}

func createNodesLogFile(logsDir string) {
	nodesFile, err := os.Create(logsDir + "nodes.log")
	Expect(err).ToNot(HaveOccurred())
	kubernetescli.RedirectOutput(nodesFile, os.Stderr, "describe", "nodes")
	defer nodesFile.Close()
}

func saveContainerLogs(clientset *kubernetes.Clientset, logsDir string, container string, pod v1.Pod) {
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: container})
	containerLogs, err := req.Stream(context.TODO())
	Expect(err).ToNot(HaveOccurred())
	defer containerLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, containerLogs)
	Expect(err).ToNot(HaveOccurred())

	logFile := logsDir + pod.Name + "-" + container + ".log"
	log.Info("Storing pod logs", "file", logFile)
	//0644
	err = ioutil.WriteFile(logFile, buf.Bytes(), os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
}
