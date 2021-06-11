package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	dataKey  = "rules.yaml"
	labelKey = "rules4tenant"
)

// Config by env
type Config struct {
	Namespace     string        `default:"loki" split_words:"true"`
	BaseDir       string        `default:"/etc/loki/rules" split_words:"true"`
	DefaultResync time.Duration `default:"30s" split_words:"true"`
}

func main() {

	//parse config form environment variables
	var c Config
	err := envconfig.Process("", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalln(err)
	}
	// creates the clientset from in-cluster config
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	listByLabelKey := func(options *metav1.ListOptions) {
		options.LabelSelector = labelKey
	}

	factory := informers.NewFilteredSharedInformerFactory(
		cli,
		c.DefaultResync,
		c.Namespace,
		listByLabelKey,
	)
	informer := factory.Core().V1().ConfigMaps().Informer()

	handler := &handler{
		BaseDir: c.BaseDir,
	}
	informer.AddEventHandler(handler)

	stopCh := make(chan struct{})

	log.Println("Start SharedInformerFactory...")
	factory.Start(stopCh)

	<-stopCh
}

type handler struct {
	BaseDir string
}

func (h *handler) OnAdd(obj interface{}) {

	cm := obj.(*corev1.ConfigMap)
	if err := h.addRule(cm); err != nil {
		log.Println(err)
	} else {
		log.Printf("add rules in %s", cm.Name)
	}
}

func (h *handler) OnDelete(obj interface{}) {

	cm := obj.(*corev1.ConfigMap)
	if err := h.deleteRule(cm); err != nil {
		log.Println(err)
	} else {
		log.Printf("delete rules in %s", cm.Name)
	}
}

func (h *handler) OnUpdate(oldObj, newObj interface{}) {

	oldCm := oldObj.(*corev1.ConfigMap)
	newCm := newObj.(*corev1.ConfigMap)

	dataChanged := oldCm.Data[dataKey] != newCm.Data[dataKey]
	tenantChanged := oldCm.Labels[labelKey] != newCm.Labels[labelKey]

	if !dataChanged && !tenantChanged {
		return
	}

	if err := h.addRule(newCm); err != nil {
		log.Println(err)
	}

	if tenantChanged {
		if err := h.deleteRule(oldCm); err != nil {
			log.Println(err)
		}
	}

	log.Printf("update rules in %s", newCm.Name)
}

func (h *handler) addRule(cm *corev1.ConfigMap) error {

	tenant := cm.Labels[labelKey]
	tenantDir := path.Join(h.BaseDir, tenant)

	//ingore if existed
	_ = os.Mkdir(tenantDir, os.ModeDir|os.FileMode(0755))

	filename := path.Join(tenantDir, dataKey)
	err := ioutil.WriteFile(filename, []byte(cm.Data[dataKey]), os.FileMode(0644))

	return err
}

func (h *handler) deleteRule(cm *corev1.ConfigMap) error {

	tenant := cm.Labels[labelKey]
	tenantDir := path.Join(h.BaseDir, tenant)
	err := os.RemoveAll(tenantDir)

	return err
}
